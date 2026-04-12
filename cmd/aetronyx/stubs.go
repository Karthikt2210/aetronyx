package aetronyx

import (
	"fmt"

	"github.com/spf13/cobra"
)

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Manage specs",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStderr(), "spec: not implemented in m1")
		return &ExitError{Code: 2}
	},
}

var checkpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "Manage checkpoints",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStderr(), "checkpoint: not implemented in m1")
		return &ExitError{Code: 2}
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStderr(), "config: not implemented in m1")
		return &ExitError{Code: 2}
	},
}

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generate shell completion",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStderr(), "completion: not implemented in m1")
		return &ExitError{Code: 2}
	},
}
