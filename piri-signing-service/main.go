package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/fil-forge/forge/internal/identity"
	"github.com/fil-forge/forge/internal/pdpsigner/signer"
	"github.com/fil-forge/forge/piri-signing-service/pkg/config"
	"github.com/fil-forge/forge/piri-signing-service/pkg/handlers"
	"github.com/fil-forge/forge/piri-signing-service/pkg/server"
)

var rootCmd = &cobra.Command{
	Use:   "signing-service",
	Short: "HTTP service for signing PDP operations on behalf of Storacha",
	Long: `A signing service that accepts PDP operation payloads via HTTP and returns
EIP-712 signatures. This service wraps the signer.Signer and provides a REST API
for piri nodes to request signatures without exposing Storacha's private key.

Phase 1 (current): Blindly signs any request (no authentication)
Phase 2 (future): UCAN authentication for registered operators
Phase 3 (future): Session key integration
Phase 4 (future): Replace cold wallet with session key`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize Viper with the command's flags
		return config.Init()
	},
	RunE: run,
}

func init() {
	// NB: on T9 keyboard 7446 spells SIGN :)
	rootCmd.Flags().String("host", config.DefaultHost, "Host to listen on")
	cobra.CheckErr(viper.BindPFlag("host", rootCmd.Flags().Lookup("host")))

	rootCmd.Flags().Int("port", config.DefaultPort, "HTTP server port")
	cobra.CheckErr(viper.BindPFlag("port", rootCmd.Flags().Lookup("port")))

	rootCmd.Flags().String("rpc-url", "", "Ethereum RPC URL")
	cobra.CheckErr(viper.BindPFlag("rpc_url", rootCmd.Flags().Lookup("rpc-url")))

	rootCmd.Flags().String("service-contract-address", "", "FilecoinWarmStorageService contract address")
	cobra.CheckErr(viper.BindPFlag("service_contract_address", rootCmd.Flags().Lookup("service-contract-address")))

	rootCmd.Flags().String("service-key", "", "Multibase-encoded private service key string")
	cobra.CheckErr(viper.BindPFlag("service_key", rootCmd.Flags().Lookup("service-key")))

	rootCmd.Flags().String("service-key-file", "", "Path to Ed25519 PEM key file for service identity")
	cobra.CheckErr(viper.BindPFlag("service_key_file", rootCmd.Flags().Lookup("service-key-file")))
	rootCmd.MarkFlagsMutuallyExclusive("service-key", "service-key-file")

	rootCmd.Flags().String("service-did", "", "A DID web that identifies this service publicly")
	cobra.CheckErr(viper.BindPFlag("service_did", rootCmd.Flags().Lookup("service-did")))

	rootCmd.Flags().String("signing-key", "", "Hex-encoded private signing key string")
	cobra.CheckErr(viper.BindPFlag("signing_key", rootCmd.Flags().Lookup("signing-key")))

	rootCmd.Flags().String("signing-key-path", "", "Path to private signing key file")
	cobra.CheckErr(viper.BindPFlag("signing_key_path", rootCmd.Flags().Lookup("signing-key-path")))

	rootCmd.Flags().String("signing-keystore-path", "", "Path to signing keystore file")
	cobra.CheckErr(viper.BindPFlag("signing_keystore_path", rootCmd.Flags().Lookup("signing-keystore-path")))

	rootCmd.Flags().String("signing-keystore-password", "", "Signing keystore password")
	cobra.CheckErr(viper.BindPFlag("signing_keystore_password", rootCmd.Flags().Lookup("signing-keystore-password")))

	rootCmd.Flags().Bool("insecure-did-resolution", false, "Enable insecure did resolution over http")
	cobra.CheckErr(viper.BindPFlag("insecure_did_resolution", rootCmd.Flags().Lookup("insecure-did-resolution")))
	cobra.CheckErr(rootCmd.Flags().MarkHidden("insecure-did-resolution"))
}

func run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Load configuration from Viper (which already has flags, env vars, and config file values)
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Load service identity
	var id identity.Identity
	switch {
	case cfg.ServiceKeyFile != "":
		id, err = identity.NewFromPEMFileWithDID(cfg.ServiceKeyFile, cfg.ServiceDID)
		if err != nil {
			return fmt.Errorf("loading service identity from file: %w", err)
		}
	case cfg.ServiceKey != "":
		id, err = identity.New(cfg.ServiceKey, cfg.ServiceDID)
		if err != nil {
			return fmt.Errorf("loading service identity: %w", err)
		}
	default:
		return fmt.Errorf("either service_key or service_key_file must be provided")
	}

	// Load private signing key
	var signingKey *ecdsa.PrivateKey
	switch {
	case cfg.SigningKey != "":
		signingKey, err = config.LoadSigningKey(cfg.SigningKey)
		if err != nil {
			return fmt.Errorf("loading signing key: %w", err)
		}
	case cfg.SigningKeyPath != "":
		signingKey, err = config.LoadSigningKeyFromFile(cfg.SigningKeyPath)
		if err != nil {
			return fmt.Errorf("loading signing key from file: %w", err)
		}
	default:
		signingKey, err = config.LoadSigningKeyFromKeystore(cfg.SigningKeystorePath, cfg.SigningKeystorePassword)
		if err != nil {
			return fmt.Errorf("loading signing keystore: %w", err)
		}
	}

	// Connect to RPC to get chain ID
	client, err := ethclient.Dial(cfg.RPCUrl)
	if err != nil {
		return fmt.Errorf("connecting to RPC endpoint: %w", err)
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("getting chain ID: %w", err)
	}

	// Create EIP-712 signer
	s := signer.NewSigner(signingKey, chainID, cfg.ContractAddr())

	// Create Echo instance
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Logger.SetLevel(log.DEBUG)

	// Create HTTP handlers
	handler := handlers.NewHandler(s)

	// Create UCAN server (UCAN 1.0 via ucantone + the protocol module)
	var ucanSvrOpts []server.Option
	if cfg.InsecureDIDResolution {
		ucanSvrOpts = append(ucanSvrOpts, server.WithInsecureDIDResolution())
	}
	ucanSrv, err := server.New(id, s, ucanSvrOpts...)
	if err != nil {
		return fmt.Errorf("creating UCAN server: %w", err)
	}

	// Setup routes — the ucantone server is an http.Handler, route POST / to it.
	e.POST("/", echo.WrapHandler(ucanSrv))
	e.GET("/healthcheck", handler.Health)
	// TODO: remove /sign/* routes after all nodes transition to UCAN invocations
	e.POST("/sign/create-dataset", handler.SignCreateDataSet)
	e.POST("/sign/add-pieces", handler.SignAddPieces)
	e.POST("/sign/schedule-piece-removals", handler.SignSchedulePieceRemovals)
	e.POST("/sign/delete-dataset", handler.SignDeleteDataSet)

	// Log startup info
	cmd.Println("Signing service starting...")
	cmd.Printf("  Service ID: %s\n", id.DID())
	cmd.Printf("  Signer address: %s\n", s.GetAddress().Hex())
	cmd.Printf("  Chain ID: %s\n", chainID.String())
	cmd.Printf("  Verifying contract: %s\n", cfg.ServiceContractAddress)
	cmd.Printf("  Host: %s\n", cfg.Host)
	cmd.Printf("  Port: %d\n", cfg.Port)

	// Start server in goroutine
	go func() {
		e.Logger.Infof("✓ Server listening on http://%s:%d", cfg.Host, cfg.Port)
		if err := e.Start(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	e.Logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	e.Logger.Info("Server stopped")
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
