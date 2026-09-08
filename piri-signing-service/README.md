# Piri Signing Service

A secure HTTP signing service for Storacha's Proof of Data Possession (PDP) operations on Filecoin.

## Why This Service Exists

The Piri Signing Service acts as a secure bridge between Storacha's cold wallet and the PDP verification system on Filecoin. Instead of exposing private keys directly to multiple piri nodes, this service provides a centralized, auditable signing endpoint that:

1. **Protects Storacha's private keys** - Keys never leave the signing service
2. **Enables distributed operations** - Multiple piri nodes can request signatures without key access
3. **Provides audit trails** - All signing operations can be logged and monitored
4. **Supports future authentication** - Designed to evolve from blind signing to authenticated operations

## Quick Start

### 1. Install the Service

```bash
go install github.com/fil-forge/piri-signing-service@latest
```

Or build from source:

```bash
git clone https://github.com/fil-forge/piri-signing-service.git
cd piri-signing-service
go build -o signing-service
```

### 2. Configure

Create a `signer.yaml` file in your working directory:

```yaml
# Network configuration
host: localhost
port: 7446  # Spells SIGN on T9 keyboard

# Ethereum RPC endpoint (required)
rpc_url: https://api.calibration.node.glif.io/rpc/v1

# Contract address (required)
service_contract_address: "0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91"

# Private key configuration (choose one)
signing_key: "0x0000000000000000000000000000000000000000"
# OR
# signing_key_path: /secure/path/to/key.hex
# OR
# signing_keystore_path: /secure/path/to/keystore.json
# signing_keystore_password: your-password
```

See `signer.yaml.example` for full configuration options.

### 3. Run

```bash
./signing-service
```

The service will start and display:
- Signer address (the address that will sign operations)
- Chain ID (detected from RPC endpoint)
- Contract address (for verification)
- Listening endpoint

## Configuration

The service supports multiple configuration methods with the following priority:

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **Configuration file** (`signer.yaml`)
4. **Default values** (only for host and port)

### Configuration Methods

#### Via Configuration File

Place a `signer.yaml` file in the current directory. See `signer.yaml.example` for a complete template.

#### Via Environment Variables

All configuration can be set via environment variables with the `SIGNING_SERVICE_` prefix:

```bash
export SIGNING_SERVICE_RPC_URL=https://api.calibration.node.glif.io/rpc/v1
export SIGNING_SERVICE_SERVICE_CONTRACT_ADDRESS=0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91
export SIGNING_SERVICE_SIGNING_KEY_PATH=/secure/path/to/key.hex
```

#### Via Command-line Flags

```bash
./signing-service \
  --rpc-url=https://api.calibration.node.glif.io/rpc/v1 \
  --contract-address=0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91 \
  --signing-key-path=/secure/path/to/key.hex
```

### Required Configuration

The following must be provided (no defaults):
- `rpc_url` - Ethereum RPC endpoint
- `service_contract_address` - FilecoinWarmStorageService contract address
- Either `signing_key` OR `signing_key_path` OR `signing_keystore_path` + `signing_keystore_password`


## Service deployment

This project contains Terraform/OpenTofu deployment scripts for the service, generated using the [storoku](https://github.com/storacha/storoku) tool. The scripts are located in the [deploy](/deploy) directory.

A GitHub Actions workflow will deploy the service when a new release is created. These deployments can also be triggered manually from a development environment.

Required environment variables **that are not secrets** are set in [deploy/.env.production.local.tpl](/deploy/.env.production.local.tpl). More specifically, `rpc_url` and `service_contract_address` are defined there (as `SIGNING_SERVICE_RPC_URL` and `SIGNING_SERVICE_SERVICE_CONTRACT_ADDRESS`, respectively).

## Container images

A push to `main` publishes to GHCR from the `Container` workflow, as
`ghcr.io/fil-forge/piri-signing-service:main` plus a `sha-<short-sha>` tag,
covering `linux/amd64` and `linux/arm64`. Release tags publish the same image
under their semver version.

## Deploying to Forge Central dev

The `main` run also asks [infra-central][] to deploy what it just published. It
dispatches a `bump-deployed-image` event carrying the manifest digest, and
infra-central's [Bump deployed image][receiver] workflow opens a pull request
pinning that digest in `terraform/envs/dev/apps/terraform.tfvars`, with
auto-merge enabled. infra-central's [Check and deploy][deploy] workflow runs
`tofu apply` on `dev/apps` on every push to its `main`, so merging that pull
request is what deploys.

The dispatch runs as the `fil-forge-bot` GitHub App and needs the
`FORGE_BOT_APP_ID` variable and the `FORGE_BOT_PRIVATE_KEY` secret. Prod pins
are promoted by hand. This is separate from the storoku deployment described
above, which targets its own environments.

[infra-central]: https://github.com/fil-forge/infra-central
[receiver]: https://github.com/fil-forge/infra-central/blob/main/.github/workflows/bump-deployed-image.yml
[deploy]: https://github.com/fil-forge/infra-central/blob/main/.github/workflows/check-and-deploy.yml

## API Endpoints

### Health Check

```bash
curl http://localhost:7446/healthcheck
```

### Sign Create Dataset

```bash
curl -X POST http://localhost:7446/sign/create-dataset \
  -H "Content-Type: application/json" \
  -d '{
    "clientDatasetId": "1",
    "payeeAddress": "0xYourPayeeAddress",
    "metadata": {
      "key1": "value1",
      "key2": "value2"
    }
  }'
```

### Sign Add Pieces

```bash
curl -X POST http://localhost:7446/sign/add-pieces \
  -H "Content-Type: application/json" \
  -d '{
    "clientDatasetId": "1",
    "pieces": ["piece1", "piece2"]
  }'
```

### Sign Schedule Piece Removals

```bash
curl -X POST http://localhost:7446/sign/schedule-piece-removals \
  -H "Content-Type: application/json" \
  -d '{
    "clientDatasetId": "1",
    "pieces": ["piece1", "piece2"]
  }'
```

### Sign Delete Dataset

```bash
curl -X POST http://localhost:7446/sign/delete-dataset \
  -H "Content-Type: application/json" \
  -d '{
    "clientDatasetId": "1"
  }'
```

## Security Roadmap

The service is designed to evolve through multiple security phases:

### Phase 1: Blind Signing (Current)
- Signs any properly formatted request
- No authentication required
- Suitable for trusted, private networks only
- **⚠️ WARNING: Do not expose to public internet**

### Phase 2: UCAN Authentication (Planned)
- Registered operators authenticate via UCAN tokens
- Each operator has specific permissions
- Audit trail of who requested each signature

### Phase 3: Session Keys (Future)
- Temporary session keys for time-limited operations
- Reduced exposure of primary signing key
- Automatic key rotation

### Phase 4: Hardware Security Module (Future)
- Cold wallet replaced with HSM or cloud KMS
- Hardware-level key protection
- Compliance with security standards

## Production Deployment

### Recommended Setup

1. **Run on a secure, isolated network** - Not exposed to public internet
2. **Use environment variables or secure key management** for sensitive configuration
3. **Enable comprehensive logging** for audit trails
4. **Monitor all signing requests** for anomalies
5. **Implement rate limiting** at the network level
6. **Use TLS/HTTPS** for all communications

### Example Systemd Service

```ini
[Unit]
Description=Piri Signing Service
After=network.target

[Service]
Type=simple
User=signing-service
WorkingDirectory=/opt/signing-service
ExecStart=/opt/signing-service/signing-service
Restart=always
RestartSec=10

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/signing-service/logs

# Environment variables
Environment="SIGNING_SERVICE_RPC_URL=https://api.node.glif.io/rpc/v1"
EnvironmentFile=/etc/signing-service/env

[Install]
WantedBy=multi-user.target
```

### Docker Deployment

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o signing-service

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/signing-service .
EXPOSE 7446
CMD ["./signing-service"]
```

```bash
docker run -d \
  -p 7446:7446 \
  -v /secure/path/to/config:/root/signer.yaml:ro \
  -v /secure/path/to/keys:/keys:ro \
  --name signing-service \
  fil-forge/piri-signing-service
```

## Monitoring

### Key Metrics to Monitor

- Request rate per endpoint
- Response times
- Error rates
- Unique requesters (when authentication is added)
- Gas price at signature time
- Contract verification failures

### Example Prometheus Metrics

The service exposes metrics at `/metrics` (when enabled):

```
signing_service_requests_total{endpoint="create_dataset"}
signing_service_request_duration_seconds{endpoint="create_dataset"}
signing_service_errors_total{endpoint="create_dataset",error="validation_failed"}
signing_service_signer_address{address="0x..."}
```

## Troubleshooting

### Common Issues

**Service won't start**
- Check all required configuration is provided
- Verify RPC endpoint is reachable
- Ensure private key file has correct permissions (600)

**"Invalid contract address"**
- Verify the contract address is a valid Ethereum address
- Ensure it matches the deployed FilecoinWarmStorageService contract

**"Failed to get chain ID"**
- Check RPC endpoint is correct and accessible
- Verify network connectivity

**"Invalid signature" from contract**
- Ensure chain ID matches the network
- Verify contract address is correct
- Check that the signer address has appropriate permissions

## Development

### Running Tests

```bash
go test ./...
```

### Building for Different Platforms

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o signing-service-linux

# macOS
GOOS=darwin GOARCH=amd64 go build -o signing-service-darwin

# Windows
GOOS=windows GOARCH=amd64 go build -o signing-service.exe
```

## Support

For issues, feature requests, or questions:
- GitHub Issues: https://github.com/fil-forge/piri-signing-service/issues
- Documentation: https://docs.storacha.com/signing-service

## License

[License details here]