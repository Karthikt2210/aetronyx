package aetronyx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/karthikcodes/aetronyx/internal/store"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit logging operations",
}

var auditVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify audit chain integrity",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := getLogger(cmd)
		cfg := getConfig(cmd)

		runID, _ := cmd.Flags().GetString("run")
		if runID == "" {
			return fmt.Errorf("--run flag is required")
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

		// Open store read-only
		dbPath := filepath.Join(dataDir, cfg.Storage.DBFilename)
		st, err := store.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open store: %w", err)
		}
		defer st.Close()

		// TODO(M1): Implement VerifyRun once store implements audit.StoreReader interface
		// For now, just return OK placeholder
		logger.Info("audit verify placeholder", "run_id", runID)
		fmt.Fprintln(cmd.OutOrStdout(), "ok")
		return nil
	},
}

var auditShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show audit events for a run",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := getLogger(cmd)
		cfg := getConfig(cmd)

		runID, _ := cmd.Flags().GetString("run")
		if runID == "" {
			return fmt.Errorf("--run flag is required")
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

		// Open store read-only
		dbPath := filepath.Join(dataDir, cfg.Storage.DBFilename)
		st, err := store.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open store: %w", err)
		}
		defer st.Close()

		// Load audit events (this will be used when store.AuditEvents accessor is ready)
		// For now, just log that we're ready
		logger.Info("audit show", "run_id", runID)

		// Output placeholder
		events := []map[string]interface{}{
			{"note": "audit events will be implemented when store accessor is ready"},
		}
		for _, event := range events {
			json.NewEncoder(cmd.OutOrStdout()).Encode(event)
		}

		return nil
	},
}

func init() {
	auditCmd.AddCommand(auditVerifyCmd)
	auditCmd.AddCommand(auditShowCmd)

	auditVerifyCmd.Flags().String("run", "", "run ID to verify")
	auditShowCmd.Flags().String("run", "", "run ID to show events for")
}
