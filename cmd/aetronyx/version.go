package aetronyx

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := getConfig(cmd)
		logger := getLogger(cmd)

		if cfg.Logging.Format == "json" {
			data := map[string]string{
				"version":  Version,
				"commit":   Commit,
				"built_at": BuiltAt,
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(data)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", Version, Commit, BuiltAt)
		logger.Debug("version command executed")
		return nil
	},
}
