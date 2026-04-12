package aetronyx

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/karthikcodes/aetronyx/internal/agent"
	"github.com/karthikcodes/aetronyx/internal/audit"
	"github.com/karthikcodes/aetronyx/internal/llm"
	"github.com/karthikcodes/aetronyx/internal/llm/anthropic"
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

		// Load or generate Ed25519 keypair
		privKey, _, err := audit.LoadOrGenerate(dataDir)
		if err != nil {
			return fmt.Errorf("failed to load or generate keypair: %w", err)
		}

		// Create audit engine with store adapter
		storeAdapter := agent.NewStoreAuditAdapter(st)
		auditEngine := audit.New(storeAdapter, privKey)

		// Create or select LLM adapter
		adapter, err := selectAdapter(cfg, logger)
		if err != nil {
			return fmt.Errorf("failed to select adapter: %w", err)
		}

		// Create agent engine
		engineCfg := agent.Config{
			Workspace:      ws,
			MaxIterations:  s.Budget.MaxIterations,
			DefaultModel:   defaultModelForAdapter(adapter),
			PlanningModel:  s.RoutingHint.PlanningModel,
			ExecutionModel: s.RoutingHint.ExecutionModel,
			SpecPath:       expanded,
			DataDir:        dataDir,
		}
		agentEngine := agent.New(st, auditEngine, adapter, engineCfg)

		// Run the loop
		ctx := context.Background()
		if err := agentEngine.Run(ctx, s); err != nil {
			logger.Error("run failed", "error", err)
			return err
		}

		logger.Info("run completed", "spec", s.Name)
		return nil
	},
}

// selectAdapter creates an LLM adapter based on configuration and environment.
// Priority: env-selected model type > config defaults > Anthropic fallback
func selectAdapter(cfg interface{ /* config.Config */ }, logger *slog.Logger) (llm.Adapter, error) {
	// For now, default to Anthropic. In M5, this will be expanded to support
	// provider routing via RoutingHint and environment configuration.
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		logger.Warn("ANTHROPIC_API_KEY not set, adapter may fail at runtime")
	}
	return anthropic.New(apiKey), nil
}

// defaultModelForAdapter returns a sensible default model for the given adapter.
func defaultModelForAdapter(adapter llm.Adapter) string {
	switch adapter.Name() {
	case "openai":
		return "gpt-4.1"
	case "ollama":
		return "llama2"
	default:
		return "claude-opus-4-6"
	}
}

func init() {
	runCmd.Flags().String("workspace", "", "workspace root")
}
