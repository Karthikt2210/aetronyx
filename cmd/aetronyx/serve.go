package aetronyx

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/karthikcodes/aetronyx/internal/audit"
	"github.com/karthikcodes/aetronyx/internal/server"
	"github.com/karthikcodes/aetronyx/internal/store"
	"github.com/karthikcodes/aetronyx/internal/telemetry"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP server with embedded UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := getConfig(cmd)
		logger := getLogger(cmd)

		// Parse serve-specific flags
		port, _ := cmd.Flags().GetInt("port")
		host, _ := cmd.Flags().GetString("host")
		allowRemote, _ := cmd.Flags().GetBool("allow-remote")

		// Override config with flags if provided
		if port > 0 {
			cfg.Server.Port = port
		}
		if host != "" {
			cfg.Server.Host = host
		}
		if allowRemote {
			cfg.Server.AllowRemote = true
		}

		// Ensure data directory exists
		dataDir := cfg.Storage.DataDir
		if dataDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			dataDir = filepath.Join(home, ".aetronyx")
		}

		if err := os.MkdirAll(dataDir, 0o700); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		// Load or generate Ed25519 keypair
		_, _, err := audit.LoadOrGenerate(dataDir)
		if err != nil {
			return fmt.Errorf("failed to load or generate keypair: %w", err)
		}
		logger.Debug("keypair loaded")

		// Load or generate auth token
		token, err := server.LoadOrGenerateToken(dataDir)
		if err != nil {
			return fmt.Errorf("failed to load or generate auth token: %w", err)
		}
		logger.Debug("auth token loaded")

		// Open store
		dbPath := filepath.Join(dataDir, cfg.Storage.DBFilename)
		st, err := store.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open store: %w", err)
		}
		defer st.Close()
		logger.Info("store opened", "path", dbPath)

		// Init telemetry
		tp, err := telemetry.Init("aetronyx", Version, cfg.Telemetry.OtelEnabled)
		if err != nil {
			return fmt.Errorf("failed to init telemetry: %w", err)
		}
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			tp.Shutdown(shutdownCtx)
		}()

		// TODO(M1): Create audit engine once store implements audit.StoreWriter interface
		// auditEngine := audit.New(st, privKey)

		// Create HTTP server
		httpServer := server.New(cfg, token, Version, logger)

		// Start server in background
		serverErrors := make(chan error, 1)
		go func() {
			serverErrors <- httpServer.Start()
		}()

		logger.Info("server started",
			"host", cfg.Server.Host,
			"port", cfg.Server.Port,
		)

		// Wait for shutdown signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		select {
		case sig := <-sigChan:
			logger.Info("received signal", "signal", sig.String())
		case err := <-serverErrors:
			if err != nil {
				logger.Error("server error", "error", err)
				return err
			}
		}

		// Graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown error", "error", err)
			return err
		}

		logger.Info("server stopped")
		return nil
	},
}

func init() {
	serveCmd.Flags().Int("port", 7777, "port to listen on")
	serveCmd.Flags().String("host", "127.0.0.1", "host to listen on")
	serveCmd.Flags().Bool("allow-remote", false, "allow remote connections")
}
