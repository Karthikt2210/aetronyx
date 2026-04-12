package repo

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// FileImpact describes a file affected by a spec change.
type FileImpact struct {
	Path            string
	Language        string
	Reason          string
	LOC             int
	ExportedSymbols []string
}

// SymbolImpact describes an exported symbol and its callers.
type SymbolImpact struct {
	Symbol    string
	DefinedIn string
	CalledBy  []string
}

// RadiusStats summarises the blast radius counts.
type RadiusStats struct {
	DirectCount      int
	ImporterCount    int
	ImporteeCount    int
	SymbolCallerCount int
	TestCount        int
}

// BlastRadiusReport is the full impact analysis for a spec.
type BlastRadiusReport struct {
	SpecHash      string
	Direct        []FileImpact
	Importers     []FileImpact
	Importees     []FileImpact
	SymbolCallers []SymbolImpact
	RelevantTests []string
	Warnings      []string
	Stats         RadiusStats
}

// ComputeBlastRadius calculates the impact of changes described by specFiles
// using the pre-built dependency graph g.
func ComputeBlastRadius(g *Graph, specFiles []string, testCommands []string) BlastRadiusReport {
	r := BlastRadiusReport{}

	// Step 1: Direct set — expand spec file globs against graph nodes.
	directSet := expandGlobs(g, specFiles)
	r.Direct = toFileImpacts(g, directSet, "direct dependency")

	// Step 2: Importers — nodes that import any direct-set node.
	importerSet := findImporters(g, directSet)
	r.Importers = toFileImpacts(g, importerSet, "imports direct file")

	// Step 3: Importees — files that direct-set nodes import.
	importeeSet := findImportees(g, directSet)
	r.Importees = toFileImpacts(g, importeeSet, "imported by direct file")

	// Step 4: Symbol callers — callers of exported symbols in direct set.
	exportedSymbols := collectExportedSymbols(g, directSet)
	r.SymbolCallers = findSymbolCallers(g, exportedSymbols)

	// Step 5: Relevant tests — test files in any impacted set.
	allImpacted := union(directSet, importerSet, importeeSet)
	r.RelevantTests = findRelevantTests(g, allImpacted)

	// Dedup: remove direct-set entries from importers/importees.
	r.Importers = dedup(r.Importers, directSet)
	r.Importees = dedup(r.Importees, directSet)

	r.Stats = RadiusStats{
		DirectCount:       len(r.Direct),
		ImporterCount:     len(r.Importers),
		ImporteeCount:     len(r.Importees),
		SymbolCallerCount: len(r.SymbolCallers),
		TestCount:         len(r.RelevantTests),
	}
	return r
}

// expandGlobs matches spec file patterns against nodes in the graph.
func expandGlobs(g *Graph, patterns []string) map[string]bool {
	matched := make(map[string]bool)
	for _, pattern := range patterns {
		for rel := range g.Nodes {
			if ok, _ := filepath.Match(pattern, rel); ok {
				matched[rel] = true
				continue
			}
			// also try matching by suffix
			if strings.HasSuffix(rel, pattern) || strings.HasSuffix(rel, "/"+pattern) {
				matched[rel] = true
			}
		}
	}
	return matched
}

// findImporters returns nodes that have an import edge pointing into the direct set.
func findImporters(g *Graph, directSet map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for _, edge := range g.Edges {
		if edge.Kind == "imports" && directSet[edge.To] {
			result[edge.From] = true
		}
	}
	return result
}

// findImportees returns nodes that the direct set imports.
func findImportees(g *Graph, directSet map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for _, edge := range g.Edges {
		if edge.Kind == "imports" && directSet[edge.From] {
			result[edge.To] = true
		}
	}
	return result
}

// collectExportedSymbols returns exported symbols from nodes in the direct set.
func collectExportedSymbols(g *Graph, directSet map[string]bool) []Symbol {
	var syms []Symbol
	for rel := range directSet {
		node, ok := g.Nodes[rel]
		if !ok {
			continue
		}
		for _, s := range node.Symbols {
			if s.Kind == KindExport {
				syms = append(syms, s)
			}
		}
	}
	return syms
}

// findSymbolCallers returns SymbolImpacts for each exported symbol.
func findSymbolCallers(g *Graph, syms []Symbol) []SymbolImpact {
	var impacts []SymbolImpact
	for _, sym := range syms {
		var callers []string
		for _, edge := range g.Edges {
			if edge.Kind == "calls" && edge.To == sym.Name {
				callers = append(callers, edge.From)
			}
		}
		impacts = append(impacts, SymbolImpact{
			Symbol:    sym.Name,
			DefinedIn: sym.File,
			CalledBy:  callers,
		})
	}
	return impacts
}

// findRelevantTests returns test files that intersect with the impacted set.
func findRelevantTests(g *Graph, impacted map[string]bool) []string {
	var tests []string
	for rel := range g.Nodes {
		if !isTestFile(rel) {
			continue
		}
		if impacted[rel] {
			tests = append(tests, rel)
			continue
		}
		// Check if test imports any impacted file
		for _, edge := range g.Edges {
			if edge.From == rel && edge.Kind == "imports" && impacted[edge.To] {
				tests = append(tests, rel)
				break
			}
		}
	}
	return tests
}

// isTestFile returns true if the path looks like a test file.
func isTestFile(path string) bool {
	base := filepath.Base(path)
	return strings.Contains(base, "_test") || strings.HasPrefix(base, "test_")
}

// toFileImpacts converts a set of node paths to FileImpact entries.
func toFileImpacts(g *Graph, paths map[string]bool, reason string) []FileImpact {
	var result []FileImpact
	for rel := range paths {
		node, ok := g.Nodes[rel]
		if !ok {
			continue
		}
		var exported []string
		for _, s := range node.Symbols {
			if s.Kind == KindExport {
				exported = append(exported, s.Name)
			}
		}
		result = append(result, FileImpact{
			Path:            rel,
			Language:        node.Lang,
			Reason:          reason,
			LOC:             node.LOC,
			ExportedSymbols: exported,
		})
	}
	return result
}

// dedup removes from impacts any entry whose path is in exclude.
func dedup(impacts []FileImpact, exclude map[string]bool) []FileImpact {
	var out []FileImpact
	for _, fi := range impacts {
		if !exclude[fi.Path] {
			out = append(out, fi)
		}
	}
	return out
}

// union merges multiple path sets into one.
func union(sets ...map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for _, s := range sets {
		for k := range s {
			result[k] = true
		}
	}
	return result
}

// FormatText renders a BlastRadiusReport as human-readable grouped text.
func FormatText(r BlastRadiusReport) string {
	var sb strings.Builder
	writeSectionText(&sb, "Direct files", r.Direct)
	writeSectionText(&sb, "Importers", r.Importers)
	writeSectionText(&sb, "Importees", r.Importees)
	if len(r.SymbolCallers) > 0 {
		fmt.Fprintf(&sb, "\nSymbol callers:\n")
		for _, sc := range r.SymbolCallers {
			fmt.Fprintf(&sb, "  %s (in %s): %s\n", sc.Symbol, sc.DefinedIn, strings.Join(sc.CalledBy, ", "))
		}
	}
	if len(r.RelevantTests) > 0 {
		fmt.Fprintf(&sb, "\nRelevant tests:\n")
		for _, t := range r.RelevantTests {
			fmt.Fprintf(&sb, "  %s\n", t)
		}
	}
	fmt.Fprintf(&sb, "\nSummary: %d direct, %d importers, %d importees, %d symbol callers, %d tests\n",
		r.Stats.DirectCount, r.Stats.ImporterCount, r.Stats.ImporteeCount,
		r.Stats.SymbolCallerCount, r.Stats.TestCount)
	return sb.String()
}

func writeSectionText(sb *strings.Builder, label string, impacts []FileImpact) {
	if len(impacts) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n%s:\n", label)
	for _, fi := range impacts {
		fmt.Fprintf(sb, "  %s (%s, %d LOC)\n", fi.Path, fi.Language, fi.LOC)
	}
}

// FormatJSON serialises a BlastRadiusReport to indented JSON.
func FormatJSON(r BlastRadiusReport) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
