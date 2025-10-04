package server

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"prospero/internal/features/topten"
)

// ServerConfig holds the configuration for both HTTP and SSH servers
type ServerConfig struct {
	Host     string
	HTTPPort string
	SSHPort  string
	ForceSSH bool // Force SSH server to start even on bunny.net
}

// isRunningInBunnyMagicContainer detects if the application is running
// inside a bunny.net Magic Container by checking for bunny.net-specific
// environment variables
func isRunningInBunnyMagicContainer() bool {
	return os.Getenv("BUNNYNET_MC_APPID") != ""
}

// StartServers starts both HTTP and SSH servers concurrently
func StartServers(ctx context.Context, config ServerConfig) error {
	fmt.Printf("\r\n🎩 Prospero Server Starting\r\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\r\n")
	fmt.Printf("Press 'q' to quit or Ctrl+C to stop\r\n\r\n")

	// Determine if SSH should be enabled
	enableSSH := true
	if isRunningInBunnyMagicContainer() && !config.ForceSSH {
		enableSSH = false
		fmt.Printf("ℹ️  SSH server disabled (running on bunny.net Magic Container)\r\n")
	}

	// Validate AGE encryption password before starting servers (only if SSH is enabled)
	if enableSSH {
		if err := topten.ValidatePassword(ctx); err != nil {
			return fmt.Errorf("AGE password validation failed: %w", err)
		}
		fmt.Printf("✅ AGE encryption password verified\r\n")
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 2)
	shutdownChan := make(chan struct{}, 2)

	// Start HTTP server in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { shutdownChan <- struct{}{} }()
		if err := StartHTTPServer(ctx, config.Host, config.HTTPPort); err != nil {
			if err != context.Canceled {
				errChan <- fmt.Errorf("HTTP server error: %w", err)
			}
		}
	}()

	// Start SSH server in a goroutine (only if enabled)
	if enableSSH {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { shutdownChan <- struct{}{} }()
			if err := StartSSHServer(ctx, config.Host, config.SSHPort); err != nil {
				if err != context.Canceled {
					errChan <- fmt.Errorf("SSH server error: %w", err)
				}
			}
		}()
	}

	// Wait for either server to fail or context to be cancelled
	select {
	case <-ctx.Done():
		// Context cancelled (Ctrl+C or SIGTERM)
		fmt.Printf("\r\n🛑 Shutting down Prospero servers...\r\n")

		// Give servers time to shut down gracefully
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			fmt.Printf("✅ All servers shut down gracefully\r\n")
		case <-shutdownCtx.Done():
			fmt.Printf("⚠️  Shutdown timeout reached\r\n")
		}

		return nil

	case err := <-errChan:
		if err != nil {
			return err
		}
	}

	return nil
}
