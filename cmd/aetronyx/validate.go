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

		specPath := args[0]
		format, _ := cmd.Flags().GetString("format")
		strict, _ := cmd.Flags().GetBool("strict")
		workspace := getWorkspace(cmd)

		// Expand path
		expanded, err := expandPath(specPath)
		if err != nil {
			return fmt.Errorf("failed to expand path: %w", err)
		}

		// M2: hardcoded adapter list for validation
		adapters := []string{
			"claude-opus-4-6",
			"claude-sonnet-4-6",
			"claude-haiku-4-5-20251001",
			"gpt-4.1",
			"gpt-4.1-mini",
			"o4-mini",
		}

		// Validate spec file
		result := spec.ValidateFile(expanded, workspace, adapters)

		if format == "json" {
			json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		} else {
			// Text output
			if result.OK && len(result.Warnings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "ok")
			} else {
				for _, e := range result.Errors {
					// Format: [FATAL] rule: message
					fmt.Fprintf(cmd.OutOrStdout(), "[FATAL] %s: %s\n", e.Rule, e.Message)
				}
				for _, w := range result.Warnings {
					// Format: [WARN] rule: message
					fmt.Fprintf(cmd.OutOrStdout(), "[WARN] %s: %s\n", w.Rule, w.Message)
				}
			}
		}

		logger.Info("spec validation completed", "path", expanded, "ok", result.OK, "errors", len(result.Errors), "warnings", len(result.Warnings))

		// Exit code logic
		if len(result.Errors) > 0 {
			return &ExitError{Code: 10}
		}
		if strict && len(result.Warnings) > 0 {
			return &ExitError{Code: 10}
		}

		return nil
	},
}

func init() {
	validateCmd.Flags().String("format", "text", "output format (text|json)")
	validateCmd.Flags().Bool("strict", false, "treat warnings as errors")
	validateCmd.Flags().String("workspace", "", "workspace root")
}
