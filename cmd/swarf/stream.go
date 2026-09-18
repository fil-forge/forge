package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fil-forge/swarf/internal/sse"
	"github.com/spf13/cobra"
)

func newStreamCommand() *cobra.Command {
	var serviceURL string
	var from string
	command := &cobra.Command{
		Use:   "stream",
		Short: "Stream revocations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if from == "" {
				from = time.Now().UTC().Format(time.RFC3339Nano)
			}
			if err := validateFrom(from); err != nil {
				return err
			}
			endpoint, err := url.Parse(serviceURL)
			if err != nil {
				return fmt.Errorf("parsing service URL: %w", err)
			}
			if !endpoint.IsAbs() || endpoint.Host == "" {
				return fmt.Errorf("service URL must be absolute: %q", serviceURL)
			}
			requestURL := strings.TrimRight(endpoint.String(), "/") + "/revocations/" + url.PathEscape(from)
			request, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, requestURL, nil)
			if err != nil {
				return fmt.Errorf("creating revocation stream request: %w", err)
			}
			request.Header.Set("Accept", "text/event-stream")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				if cmd.Context().Err() != nil {
					return nil
				}
				return fmt.Errorf("opening revocation stream: %w", err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				return fmt.Errorf("opening revocation stream: unexpected status %s", response.Status)
			}
			if err := writeStreamEvents(cmd, response.Body); err != nil {
				if cmd.Context().Err() != nil {
					return nil
				}
				return err
			}
			return nil
		},
	}
	command.Flags().StringVar(&serviceURL, "service-url", defaultServiceURL, "Swarf service URL")
	command.Flags().StringVar(&from, "from", "", "stream revocations recorded on or after this time: 0, RFC3339, or RFC3339Nano (default: now)")
	return command
}

func validateFrom(value string) error {
	if value == "0" {
		return nil
	}
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		return fmt.Errorf("invalid from timestamp: %w", err)
	}
	return nil
}

func writeStreamEvents(cmd *cobra.Command, body io.Reader) error {
	events := sse.NewScanner(body)
	for events.Scan() {
		switch events.Event() {
		case "revocation":
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), events.Data()); err != nil {
				return fmt.Errorf("writing revocation event: %w", err)
			}
		case "error":
			// The service reports a stream it cannot continue as an error
			// event and then closes. Ignoring it ends the command at exit 0,
			// indistinguishable from reaching the end of the data.
			return fmt.Errorf("revocation stream: %s", events.Data())
		}
	}
	if err := events.Err(); err != nil {
		return fmt.Errorf("reading revocation stream: %w", err)
	}
	return nil
}
