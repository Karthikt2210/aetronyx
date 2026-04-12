package repo

import (
	"bufio"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Node represents a source file in the dependency graph.
type Node struct {
	Path    string
	Lang    string
	LOC     int
	Symbols []Symbol
	Imports []string
}

// Edge represents a directed relationship between two nodes.
// Kind is one of: "imports", "defines", "calls", "extends", "implements".
type Edge struct {
	From string
	To   string
	Kind string
}

// Graph holds the full dependency graph of a workspace.
type Graph struct {
	Nodes map[string]*Node
	Edges []Edge
}

// skipDirs are directories skipped during workspace walk.
var skipDirs = map[string]bool{
	".git": true, ".aetronyx": true, "node_modules": true,
	"vendor": true, "testdata": true,
}

// Build walks workspace, parses each supported file, and builds the dependency graph.
func Build(workspace string) (*Graph, error) {
	g := &Graph{Nodes: make(map[string]*Node)}

	err := filepath.WalkDir(workspace, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		lang := DetectLanguage(path)
		if lang == "" {
			return nil
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return nil
		}
		imports, symbols, err := ParseFile(path, lang)
		if err != nil {
			return nil // best-effort: skip unparseable files
		}
		loc := countLines(path)
		g.Nodes[rel] = &Node{
			Path:    rel,
			Lang:    string(lang),
			LOC:     loc,
			Symbols: symbols,
			Imports: imports,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", workspace, err)
	}

	moduleName := readModuleName(workspace)
	addImportEdges(g, workspace, moduleName)
	return g, nil
}

// addImportEdges resolves import paths to workspace files and adds import edges.
func addImportEdges(g *Graph, workspace, moduleName string) {
	for rel, node := range g.Nodes {
		for _, imp := range node.Imports {
			to := resolveImport(imp, workspace, moduleName, g)
			if to != "" && to != rel {
				g.Edges = append(g.Edges, Edge{From: rel, To: to, Kind: "imports"})
			}
		}
	}
}

// resolveImport attempts to find a workspace-relative path for an import string.
func resolveImport(imp, workspace, moduleName string, g *Graph) string {
	// Strip module prefix for Go imports
	suffix := imp
	if moduleName != "" && strings.HasPrefix(imp, moduleName+"/") {
		suffix = imp[len(moduleName)+1:]
	}

	// Check direct match as a node path
	if _, ok := g.Nodes[suffix]; ok {
		return suffix
	}

	// Look for suffix as a directory prefix among known nodes
	for rel := range g.Nodes {
		if strings.HasPrefix(rel, suffix+"/") || rel == suffix {
			return rel
		}
		// Match base directory of the node
		dir := filepath.Dir(rel)
		if dir == suffix || dir == "."+string(filepath.Separator)+suffix {
			return rel
		}
	}

	// For single-name imports (Python, etc.), look for <imp>.py or <imp>/<imp>.go etc.
	for rel := range g.Nodes {
		base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
		if base == imp {
			return rel
		}
	}

	return ""
}

// readModuleName reads the module name from go.mod in the workspace root.
func readModuleName(workspace string) string {
	f, err := os.Open(filepath.Join(workspace, "go.mod"))
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// countLines counts the number of lines in a file.
func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count
}

// CacheKey returns a sha256 fingerprint of all file modification times and sizes.
func (g *Graph) CacheKey(workspace string) (string, error) {
	h := sha256.New()
	for _, node := range g.Nodes {
		info, err := os.Stat(filepath.Join(workspace, node.Path))
		if err != nil {
			continue
		}
		fmt.Fprintf(h, "%s:%d:%d\n", node.Path, info.Size(), info.ModTime().UnixNano())
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// SaveCache persists the graph to a file using gob encoding.
func SaveCache(path string, g *Graph) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create cache: %w", err)
	}
	if err := gob.NewEncoder(f).Encode(g); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("encode graph: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close cache: %w", err)
	}
	return os.Rename(tmp, path)
}

// LoadCache reads a graph from a gob-encoded cache file.
func LoadCache(path string) (*Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cache %s: %w", path, err)
	}
	defer f.Close()
	var g Graph
	if err := gob.NewDecoder(f).Decode(&g); err != nil {
		return nil, fmt.Errorf("decode graph: %w", err)
	}
	return &g, nil
}
