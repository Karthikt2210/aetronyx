package aetronyx

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/karthikcodes/aetronyx/internal/config"
)

// Version info set by ldflags at build time.
var (
	Version = "v0.1.0-m1"
	Commit  = "dev"
	BuiltAt = "dev"
)

// ExitError is used to exit with a specific code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit code %d", e.Code)
}

var (
	cfgPath   string
	logLevel  string
	logFormat string
	workspace string
	cwd       string
)

var rootCmd = &cobra.Command{
	Use:   "aetronyx",
	Short: "AI agent loop with governance, audit, and cost control",
	Long:  `Aetronyx is a single Go binary that executes spec-driven AI agent loops with built-in cost intelligence, audit trails, and governance.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Capture cwd early
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		// Build flags map for config loader
		flags := make(map[string]string)
		if cfgPath != "" {
			flags["config"] = cfgPath
		}
		if logLevel != "" {
			flags["log-level"] = logLevel
		}
		if logFormat != "" {
			flags["log-format"] = logFormat
		}
		if workspace != "" {
			flags["workspace"] = workspace
		}

		// Load config
		cfg, warnings, err := config.Load(flags)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		if len(warnings) > 0 {
			for _, w := range warnings {
				fmt.Fprintf(os.Stderr, "warning: %s\n", w)
			}
		}

		// Validate config
		if errs := config.Validate(cfg); len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "config error: %s\n", e)
			}
			return fmt.Errorf("configuration validation failed")
		}

		// Set up logger
		var logHandler slog.Handler
		switch cfg.Logging.Format {
		case "json":
			logHandler = slog.NewJSONHandler(os.Stderr, nil)
		default:
			logHandler = slog.NewTextHandler(os.Stderr, nil)
		}

		logger := slog.New(logHandler)
		logger = logger.With(slog.String("version", Version))

		// Store in context so subcommands can access
		ctx := context.WithValue(cmd.Context(), "config", &cfg)
		ctx = context.WithValue(ctx, "logger", logger)
		ctx = context.WithValue(ctx, "cwd", cwd)
		cmd.SetContext(ctx)

		return nil
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path (default: ~/.aetronyx/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug|info|warn|error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log format (text|json)")
	rootCmd.PersistentFlags().StringVar(&workspace, "workspace", "", "workspace root (default: cwd)")

	// Register subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(auditCmd)

	// Stubs for unimplemented commands
	rootCmd.AddCommand(specCmd)
	rootCmd.AddCommand(checkpointCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(completionCmd)
}

// Execute runs the root command.
func Execute() error {
	// Handle ExitError to propagate exit code
	err := rootCmd.Execute()
	if err == nil {
		return nil
	}

	// If it's an ExitError, extract the code (it will be handled by main)
	// For now, just return the error and let main handle it
	return err
}

// getConfig retrieves config from command context.
func getConfig(cmd *cobra.Command) *config.Config {
	return cmd.Context().Value("config").(*config.Config)
}

// getLogger retrieves logger from command context.
func getLogger(cmd *cobra.Command) *slog.Logger {
	return cmd.Context().Value("logger").(*slog.Logger)
}

// getCwd retrieves captured cwd from command context.
func getCwd(cmd *cobra.Command) string {
	v := cmd.Context().Value("cwd")
	if v == nil {
		return ""
	}
	return v.(string)
}

// getWorkspace resolves workspace from flags, global flag, or cwd.
func getWorkspace(cmd *cobra.Command) string {
	// Check local flag first
	if ws, _ := cmd.Flags().GetString("workspace"); ws != "" {
		return ws
	}
	// Check global flag
	if workspace != "" {
		return workspace
	}
	// Fall back to cwd
	return getCwd(cmd)
}

// expandPath handles tilde expansion and relative paths.
func expandPath(p string) (string, error) {
	if p == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}

	if p[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, p[1:]), nil
	}

	if !filepath.IsAbs(p) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, p), nil
	}

	return p, nil
}
