package aetronyx

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/karthikcodes/aetronyx/internal/repo"
	"github.com/karthikcodes/aetronyx/internal/spec"
	"github.com/karthikcodes/aetronyx/internal/store"
)

// spec subcommands (specCmd is the root command defined in stubs.go)

// spec init — create a template spec file
var specInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new spec from template",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := getLogger(cmd)
		yes, _ := cmd.Flags().GetBool("yes")

		// Write template to example.spec.yaml
		template := spec.DefaultTemplate()
		outPath := filepath.Join(getCwd(cmd), "example.spec.yaml")

		if err := os.WriteFile(outPath, template, 0o644); err != nil {
			return fmt.Errorf("spec init: write file: %w", err)
		}

		logger.Info("spec template created", "path", outPath)

		// Offer to add .aetronyx/ to .gitignore
		gitignorePath := filepath.Join(getCwd(cmd), ".gitignore")
		if _, err := os.Stat(gitignorePath); err == nil {
			// .gitignore exists
			data, _ := os.ReadFile(gitignorePath)
			content := string(data)

			if !strings.Contains(content, ".aetronyx/") {
				if !yes {
					fmt.Fprintf(os.Stderr, "Add .aetronyx/ to .gitignore? (y/n) ")
					scanner := bufio.NewScanner(os.Stdin)
					if scanner.Scan() && strings.ToLower(scanner.Text()) == "y" {
						yes = true
					}
				}

				if yes {
					if !strings.HasSuffix(content, "\n") {
						content += "\n"
					}
					content += ".aetronyx/\n"
					if err := os.WriteFile(gitignorePath, []byte(content), 0o644); err != nil {
						logger.Warn("failed to update .gitignore", "error", err)
					} else {
						logger.Info("updated .gitignore")
					}
				}
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "created example.spec.yaml\n")
		return nil
	},
}

// spec new — create a new named spec file
var specNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new spec file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := getLogger(cmd)
		name := args[0]

		// Validate name format: ^[a-z0-9][a-z0-9-]{0,63}$
		nameRe := regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
		if !nameRe.MatchString(name) {
			return fmt.Errorf("invalid spec name %q: must be lowercase alphanumeric with hyphens, 1-64 chars", name)
		}

		// Create spec with name pre-filled
		specYaml := fmt.Sprintf(`spec_version: "1"
name: %s
intent: Describe this task.
budget:
  max_cost_usd: 10.0
  max_iterations: 50
acceptance_criteria:
  - given: input
    when: processing
    then: output is valid
invariants: []
out_of_scope: []
dependencies:
  files: []
  services: []
  apis: []
test_contracts: []
approval_gates: []
routing_hint: {}
metadata: {}
`, name)

		outPath := filepath.Join(getCwd(cmd), name+".spec.yaml")
		if err := os.WriteFile(outPath, []byte(specYaml), 0o644); err != nil {
			return fmt.Errorf("spec new: write file: %w", err)
		}

		logger.Info("spec created", "path", outPath)
		fmt.Fprintf(cmd.OutOrStdout(), "created %s.spec.yaml\n", name)
		return nil
	},
}

// spec list — list all runs (specs grouped by run)
var specListCmd = &cobra.Command{
	Use:   "list",
	Short: "List spec runs",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := getLogger(cmd)
		cfg := getConfig(cmd)
		ctx := cmd.Context()

		// Open store read-only
		dataDir := cfg.Storage.DataDir
		if dataDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			dataDir = filepath.Join(home, ".aetronyx")
		}

		dbPath := filepath.Join(dataDir, cfg.Storage.DBFilename)
		st, err := store.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open store: %w", err)
		}
		defer st.Close()

		// List all runs
		runs, err := st.ListRuns(ctx, store.ListRunsFilter{})
		if err != nil {
			return fmt.Errorf("spec list: query: %w", err)
		}

		// Print header
		fmt.Fprintf(cmd.OutOrStdout(), "%-32s %-32s %-20s\n", "SPEC_NAME", "RUN_ID", "CREATED_AT")
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", strings.Repeat("-", 32), strings.Repeat("-", 32), strings.Repeat("-", 20))

		// Print each run
		for _, r := range runs {
			startedAt := ""
			if r.StartedAt > 0 {
				startedAt = fmt.Sprintf("%d", r.StartedAt)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-32s %-32s %-20s\n", r.SpecName, r.ID, startedAt)
		}

		logger.Info("spec list completed", "count", len(runs))
		return nil
	},
}

// spec blast-radius — compute blast radius for a spec
var specBlastRadiusCmd = &cobra.Command{
	Use:   "blast-radius <spec>",
	Short: "Compute blast radius for a spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := getLogger(cmd)
		specPath := args[0]
		format, _ := cmd.Flags().GetString("format")
		workspace := getWorkspace(cmd)

		// Expand path
		expanded, err := expandPath(specPath)
		if err != nil {
			return fmt.Errorf("failed to expand path: %w", err)
		}

		// Parse spec
		s, err := spec.Parse(expanded)
		if err != nil {
			return fmt.Errorf("spec parse failed: %w", err)
		}

		// Validate spec first
		adapters := []string{
			"claude-opus-4-6",
			"claude-sonnet-4-6",
			"claude-haiku-4-5-20251001",
			"gpt-4.1",
			"gpt-4.1-mini",
			"o4-mini",
		}
		result := spec.Validate(s, workspace, adapters)
		if !result.OK {
			return fmt.Errorf("spec validation failed: %d errors", len(result.Errors))
		}

		// Build graph
		graph, err := repo.Build(workspace)
		if err != nil {
			return fmt.Errorf("repo graph build failed: %w", err)
		}

		// Extract file dependencies and test contracts
		specFiles := s.Dependencies.Files
		testCommands := make([]string, len(s.TestContracts))
		for i, tc := range s.TestContracts {
			testCommands[i] = tc.Command
		}

		// Compute blast radius
		radius := repo.ComputeBlastRadius(graph, specFiles, testCommands)

		// Output
		if format == "json" {
			json.NewEncoder(cmd.OutOrStdout()).Encode(radius)
		} else {
			// Text output
			fmt.Fprintf(cmd.OutOrStdout(), "Blast Radius Report\n")
			fmt.Fprintf(cmd.OutOrStdout(), "===================\n\n")

			fmt.Fprintf(cmd.OutOrStdout(), "Direct: %d files\n", len(radius.Direct))
			for _, f := range radius.Direct {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%d LOC)\n", f.Path, f.LOC)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nImporters: %d files\n", len(radius.Importers))
			for _, f := range radius.Importers {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%d LOC)\n", f.Path, f.LOC)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nImportees: %d files\n", len(radius.Importees))
			for _, f := range radius.Importees {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%d LOC)\n", f.Path, f.LOC)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nRelevant Tests: %d\n", len(radius.RelevantTests))
			for _, t := range radius.RelevantTests {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", t)
			}
		}

		logger.Info("blast radius computed", "spec", s.Name, "direct", len(radius.Direct))
		return nil
	},
}

func init() {
	// Add subcommands to spec
	specCmd.AddCommand(specInitCmd)
	specCmd.AddCommand(specNewCmd)
	specCmd.AddCommand(specListCmd)
	specCmd.AddCommand(specBlastRadiusCmd)

	// Flags
	specInitCmd.Flags().Bool("yes", false, "skip confirmation for .gitignore update")
	specBlastRadiusCmd.Flags().String("format", "text", "output format (text|json)")
	specBlastRadiusCmd.Flags().String("workspace", "", "workspace root")
}
