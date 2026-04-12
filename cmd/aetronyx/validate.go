package aetronyx

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/karthikcodes/aetronyx/internal/spec"
)

var validateCmd = &cobra.Command{
	Use:   "validate <spec>",
	Short: "Validate a spec file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := getLogger(cmd)
		cfg := getConfig(cmd)

		specPath := args[0]

		// Expand path
		expanded, err := expandPath(specPath)
		if err != nil {
			return fmt.Errorf("failed to expand path: %w", err)
		}

		// Parse spec
		_, err = spec.Parse(expanded)
		if err != nil {
			if cfg.Logging.Format == "json" {
				errResp := map[string]interface{}{
					"error": map[string]string{
						"code":    "spec.invalid",
						"message": err.Error(),
					},
				}
				json.NewEncoder(cmd.OutOrStdout()).Encode(errResp)
			} else {
				fmt.Fprintf(cmd.OutOrStderr(), "error: spec invalid: %v\n", err)
			}
			return &ExitError{Code: 10}
		}

		logger.Info("spec validation passed", "path", expanded)
		fmt.Fprintln(cmd.OutOrStdout(), "ok")
		return nil
	},
}
