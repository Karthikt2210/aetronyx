package aetronyx

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/karthikcodes/aetronyx/internal/audit"
	"github.com/karthikcodes/aetronyx/internal/spec"
	"github.com/karthikcodes/aetronyx/internal/store"
)

var runCmd = &cobra.Command{
	Use:   "run <spec>",
	Short: "Execute a spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := getLogger(cmd)
		cfg := getConfig(cmd)

		specPath := args[0]

		// Resolve workspace
		ws := getWorkspace(cmd)
		if ws == "" {
			var err error
			ws, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
		}

		// Expand spec path
		expanded, err := expandPath(specPath)
		if err != nil {
			return fmt.Errorf("failed to expand path: %w", err)
		}

		// Parse spec
		s, err := spec.Parse(expanded)
		if err != nil {
			logger.Error("spec parse failed", "path", expanded, "error", err)
			return &ExitError{Code: 10}
		}

		logger.Info("spec loaded", "name", s.Name, "workspace", ws)

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

		// Load Ed25519 keypair (will be used by audit engine)
		if _, _, err = audit.LoadOrGenerate(dataDir); err != nil {
			return fmt.Errorf("failed to load or generate keypair: %w", err)
		}

		// Open store
		dbPath := filepath.Join(dataDir, cfg.Storage.DBFilename)
		st, err := store.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open store: %w", err)
		}
		defer st.Close()

		// TODO(M1): Create audit engine once store implements audit.StoreWriter interface
		// auditEngine := audit.New(st, privKey)

		// TODO(M1): Create agent engine and run the loop
		// For now, just log that we're ready
		logger.Info("run ready",
			"spec", s.Name,
			"workspace", ws,
		)

		// Placeholder: emit a run created event and exit
		// This will be replaced by the actual agent loop in M1

		fmt.Fprintf(cmd.OutOrStdout(), "spec execution not yet implemented\n")
		return &ExitError{Code: 1}
	},
}

func init() {
	runCmd.Flags().String("workspace", "", "workspace root")
}
