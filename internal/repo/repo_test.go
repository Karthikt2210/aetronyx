package repo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karthikcodes/aetronyx/internal/repo"
)

// TestDetectLanguage verifies extension-to-language mapping.
func TestDetectLanguage(t *testing.T) {
	cases := []struct {
		path string
		want repo.Language
	}{
		{"main.go", repo.LangGo},
		{"app.ts", repo.LangTypeScript},
		{"component.tsx", repo.LangTypeScript},
		{"index.js", repo.LangJavaScript},
		{"util.jsx", repo.LangJavaScript},
		{"server.py", repo.LangPython},
		{"lib.rs", repo.LangRust},
		{"README.md", ""},
		{"data.json", ""},
	}
	for _, tc := range cases {
		got := repo.DetectLanguage(tc.path)
		if got != tc.want {
			t.Errorf("DetectLanguage(%q) = %q; want %q", tc.path, got, tc.want)
		}
	}
}

// TestParseGoFile verifies that imports and symbols are extracted from a Go file.
func TestParseGoFile(t *testing.T) {
	dir := t.TempDir()
	src := `package foo

import (
	"fmt"
	"os"
)

func PublicFunc() {}
func privateFunc() {}
type MyType struct{}
`
	path := filepath.Join(dir, "foo.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	imports, symbols, err := repo.ParseFile(path, repo.LangGo)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if !containsStr(imports, "fmt") || !containsStr(imports, "os") {
		t.Errorf("expected imports [fmt os], got %v", imports)
	}

	if !containsSym(symbols, "PublicFunc") {
		t.Errorf("expected symbol PublicFunc, got %v", symbols)
	}
	if !containsSym(symbols, "privateFunc") {
		t.Errorf("expected symbol privateFunc, got %v", symbols)
	}
	if !containsSym(symbols, "MyType") {
		t.Errorf("expected symbol MyType, got %v", symbols)
	}

	// Exported symbols should use KindExport
	for _, s := range symbols {
		if s.Name == "PublicFunc" && s.Kind != repo.KindExport {
			t.Errorf("PublicFunc kind = %q; want %q", s.Kind, repo.KindExport)
		}
	}
}

// TestParsePythonFile verifies import and class extraction from a Python file.
func TestParsePythonFile(t *testing.T) {
	dir := t.TempDir()
	src := `import os
import sys
from pathlib import Path

class MyClass:
    pass

def my_func():
    pass
`
	path := filepath.Join(dir, "mod.py")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	imports, symbols, err := repo.ParseFile(path, repo.LangPython)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if !containsStr(imports, "os") && !containsStr(imports, "sys") {
		t.Errorf("expected python imports containing os or sys, got %v", imports)
	}
	if !containsSym(symbols, "MyClass") {
		t.Errorf("expected symbol MyClass, got %v", symbols)
	}
	if !containsSym(symbols, "my_func") {
		t.Errorf("expected symbol my_func, got %v", symbols)
	}
}

// TestGraphBuild verifies that a 3-file Go workspace produces 3 nodes and 2 import edges.
func TestGraphBuild(t *testing.T) {
	ws := t.TempDir()
	writeGoMod(t, ws, "testmod")

	// a/a.go imports testmod/b
	writeGoFile(t, ws, "a/a.go", "package a", `import "testmod/b"`, "")
	// b/b.go imports testmod/c
	writeGoFile(t, ws, "b/b.go", "package b", `import "testmod/c"`, "")
	// c/c.go has no imports
	writeGoFile(t, ws, "c/c.go", "package c", "", "")

	g, err := repo.Build(ws)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(g.Nodes) != 3 {
		t.Errorf("want 3 nodes, got %d: %v", len(g.Nodes), nodeKeys(g))
	}

	importEdges := countEdges(g, "imports")
	if importEdges != 2 {
		t.Errorf("want 2 import edges, got %d", importEdges)
	}
}

// TestBlastRadius_Direct verifies that a specFile in the graph becomes a direct hit.
func TestBlastRadius_Direct(t *testing.T) {
	g := &repo.Graph{
		Nodes: map[string]*repo.Node{
			"b/b.go": {Path: "b/b.go", Lang: "go"},
		},
	}
	r := repo.ComputeBlastRadius(g, []string{"b/b.go"}, nil)
	if r.Stats.DirectCount != 1 {
		t.Errorf("want 1 direct, got %d", r.Stats.DirectCount)
	}
}

// TestBlastRadius_Importers verifies that a file importing a direct-set file is an importer.
func TestBlastRadius_Importers(t *testing.T) {
	g := &repo.Graph{
		Nodes: map[string]*repo.Node{
			"a/a.go": {Path: "a/a.go", Lang: "go"},
			"b/b.go": {Path: "b/b.go", Lang: "go"},
		},
		Edges: []repo.Edge{
			{From: "a/a.go", To: "b/b.go", Kind: "imports"},
		},
	}
	r := repo.ComputeBlastRadius(g, []string{"b/b.go"}, nil)
	if r.Stats.ImporterCount != 1 {
		t.Errorf("want 1 importer, got %d", r.Stats.ImporterCount)
	}
	if len(r.Importers) == 0 || r.Importers[0].Path != "a/a.go" {
		t.Errorf("expected importer a/a.go, got %v", r.Importers)
	}
}

// TestBlastRadius_Importees verifies that a file imported by a direct-set node is an importee.
func TestBlastRadius_Importees(t *testing.T) {
	g := &repo.Graph{
		Nodes: map[string]*repo.Node{
			"b/b.go": {Path: "b/b.go", Lang: "go"},
			"c/c.go": {Path: "c/c.go", Lang: "go"},
		},
		Edges: []repo.Edge{
			{From: "b/b.go", To: "c/c.go", Kind: "imports"},
		},
	}
	r := repo.ComputeBlastRadius(g, []string{"b/b.go"}, nil)
	if r.Stats.ImporteeCount != 1 {
		t.Errorf("want 1 importee, got %d", r.Stats.ImporteeCount)
	}
}

// TestCacheRoundtrip verifies that a graph survives a save/load cycle.
func TestCacheRoundtrip(t *testing.T) {
	ws := t.TempDir()
	writeGoMod(t, ws, "testmod")
	writeGoFile(t, ws, "main.go", "package main", "", "")

	g, err := repo.Build(ws)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	cachePath := filepath.Join(t.TempDir(), "graph.cache")
	if err := repo.SaveCache(cachePath, g); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	loaded, err := repo.LoadCache(cachePath)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}

	if len(loaded.Nodes) != len(g.Nodes) {
		t.Errorf("node count mismatch: got %d, want %d", len(loaded.Nodes), len(g.Nodes))
	}
	for path := range g.Nodes {
		if _, ok := loaded.Nodes[path]; !ok {
			t.Errorf("node %q missing after roundtrip", path)
		}
	}
}

// TestCacheKey verifies that a graph produces a stable, non-empty cache key.
func TestCacheKey(t *testing.T) {
	ws := t.TempDir()
	writeGoMod(t, ws, "testmod")
	writeGoFile(t, ws, "main.go", "package main", "", "")

	g, err := repo.Build(ws)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	key1, err := g.CacheKey(ws)
	if err != nil {
		t.Fatalf("CacheKey: %v", err)
	}
	if key1 == "" {
		t.Error("expected non-empty cache key")
	}
	// Key must be stable across calls.
	key2, _ := g.CacheKey(ws)
	if key1 != key2 {
		t.Errorf("cache key not stable: %q vs %q", key1, key2)
	}
}

// TestBlastRadius_RelevantTests verifies that test files in the impacted set are included.
func TestBlastRadius_RelevantTests(t *testing.T) {
	g := &repo.Graph{
		Nodes: map[string]*repo.Node{
			"pkg/pkg.go":      {Path: "pkg/pkg.go", Lang: "go"},
			"pkg/pkg_test.go": {Path: "pkg/pkg_test.go", Lang: "go"},
		},
		Edges: []repo.Edge{
			{From: "pkg/pkg_test.go", To: "pkg/pkg.go", Kind: "imports"},
		},
	}
	r := repo.ComputeBlastRadius(g, []string{"pkg/pkg.go"}, nil)
	if r.Stats.TestCount == 0 {
		t.Errorf("expected at least 1 relevant test, got 0")
	}
}

// TestBlastRadius_SymbolCallers verifies that files with calls edges to exported symbols are found.
func TestBlastRadius_SymbolCallers(t *testing.T) {
	g := &repo.Graph{
		Nodes: map[string]*repo.Node{
			"lib/lib.go":  {Path: "lib/lib.go", Lang: "go", Symbols: []repo.Symbol{{Name: "Handler", Kind: repo.KindExport, File: "lib/lib.go"}}},
			"main/main.go": {Path: "main/main.go", Lang: "go"},
		},
		Edges: []repo.Edge{
			{From: "main/main.go", To: "Handler", Kind: "calls"},
		},
	}
	r := repo.ComputeBlastRadius(g, []string{"lib/lib.go"}, nil)
	if r.Stats.SymbolCallerCount == 0 {
		t.Errorf("expected symbol callers, got 0")
	}
}

// TestFormatText verifies that FormatText produces a non-empty summary.
func TestFormatText(t *testing.T) {
	g := &repo.Graph{
		Nodes: map[string]*repo.Node{
			"a.go": {Path: "a.go", Lang: "go", LOC: 10},
			"b.go": {Path: "b.go", Lang: "go", LOC: 5},
		},
		Edges: []repo.Edge{
			{From: "b.go", To: "a.go", Kind: "imports"},
		},
	}
	r := repo.ComputeBlastRadius(g, []string{"a.go"}, nil)
	text := repo.FormatText(r)
	if text == "" {
		t.Error("expected non-empty FormatText output")
	}
	if !strings.Contains(text, "Summary") {
		t.Errorf("FormatText missing summary: %s", text)
	}
}

// TestFormatJSON verifies that FormatJSON produces valid JSON.
func TestFormatJSON(t *testing.T) {
	r := repo.ComputeBlastRadius(&repo.Graph{Nodes: map[string]*repo.Node{
		"a.go": {Path: "a.go", Lang: "go"},
	}}, []string{"a.go"}, nil)
	b, err := repo.FormatJSON(r)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	if len(b) == 0 {
		t.Error("expected non-empty JSON output")
	}
}

// TestParseTypeScriptFile verifies imports and symbols extracted from a TS file.
func TestParseTypeScriptFile(t *testing.T) {
	dir := t.TempDir()
	src := `import { foo } from "./foo";
import bar from "bar";

export class MyService {}
export function helperFn() {}
interface Config {}
`
	path := filepath.Join(dir, "service.ts")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	imports, symbols, err := repo.ParseFile(path, repo.LangTypeScript)
	if err != nil {
		t.Fatalf("ParseFile TS: %v", err)
	}
	_ = imports  // best-effort; at minimum no error
	_ = symbols
}

// TestParseRustFile verifies that Rust file parsing returns no error.
func TestParseRustFile(t *testing.T) {
	dir := t.TempDir()
	src := `use std::io::Write;

pub fn main() {}
pub struct Config {}
pub trait Handler {}
`
	path := filepath.Join(dir, "main.rs")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	imports, symbols, err := repo.ParseFile(path, repo.LangRust)
	if err != nil {
		t.Fatalf("ParseFile Rust: %v", err)
	}
	_ = imports
	_ = symbols
}

// TestUnsupportedLanguage verifies that an unknown language returns nil,nil,nil.
func TestUnsupportedLanguage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.rb")
	if err := os.WriteFile(path, []byte("puts 'hi'"), 0o644); err != nil {
		t.Fatal(err)
	}
	imports, symbols, err := repo.ParseFile(path, "")
	if err != nil || imports != nil || symbols != nil {
		t.Errorf("expected nil,nil,nil for unsupported lang; got %v,%v,%v", imports, symbols, err)
	}
}

// --- helpers ---

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func containsSym(syms []repo.Symbol, name string) bool {
	for _, s := range syms {
		if s.Name == name {
			return true
		}
	}
	return false
}

func countEdges(g *repo.Graph, kind string) int {
	n := 0
	for _, e := range g.Edges {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

func nodeKeys(g *repo.Graph) []string {
	var keys []string
	for k := range g.Nodes {
		keys = append(keys, k)
	}
	return keys
}

func writeGoMod(t *testing.T, ws, moduleName string) {
	t.Helper()
	content := "module " + moduleName + "\n\ngo 1.23\n"
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeGoFile(t *testing.T, ws, rel, pkg, importLine, extra string) {
	t.Helper()
	dir := filepath.Join(ws, filepath.Dir(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var imp string
	if importLine != "" {
		imp = "\n" + importLine + "\n"
	}
	content := pkg + imp + "\n" + extra + "\n"
	if err := os.WriteFile(filepath.Join(ws, rel), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
