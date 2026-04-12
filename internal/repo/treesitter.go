package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	sitter "github.com/smacker/go-tree-sitter"
	sitter_go "github.com/smacker/go-tree-sitter/golang"
	sitter_js "github.com/smacker/go-tree-sitter/javascript"
	sitter_py "github.com/smacker/go-tree-sitter/python"
	sitter_rs "github.com/smacker/go-tree-sitter/rust"
	sitter_ts "github.com/smacker/go-tree-sitter/typescript/tsx"
)

// Language represents a supported programming language.
type Language string

const (
	LangGo         Language = "go"
	LangTypeScript Language = "typescript"
	LangJavaScript Language = "javascript"
	LangPython     Language = "python"
	LangRust       Language = "rust"
)

// Symbol kind constants.
const (
	KindFunction  = "function"
	KindClass     = "class"
	KindInterface = "interface"
	KindType      = "type"
	KindExport    = "export"
)

// Symbol is a named definition found in a source file.
type Symbol struct {
	Name string
	Kind string
	File string
	Line int
}

// DetectLanguage returns the Language for a given file path based on extension.
func DetectLanguage(path string) Language {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return LangGo
	case ".ts", ".tsx":
		return LangTypeScript
	case ".js", ".jsx":
		return LangJavaScript
	case ".py":
		return LangPython
	case ".rs":
		return LangRust
	default:
		return ""
	}
}

// sitterLang maps a Language to its tree-sitter grammar.
func sitterLang(lang Language) *sitter.Language {
	switch lang {
	case LangGo:
		return sitter_go.GetLanguage()
	case LangTypeScript:
		return sitter_ts.GetLanguage()
	case LangJavaScript:
		return sitter_js.GetLanguage()
	case LangPython:
		return sitter_py.GetLanguage()
	case LangRust:
		return sitter_rs.GetLanguage()
	default:
		return nil
	}
}

// ParseFile extracts imports and symbol definitions from a source file.
// Returns nil, nil, nil for unsupported languages or unparseable files.
func ParseFile(path string, lang Language) (imports []string, symbols []Symbol, err error) {
	sl := sitterLang(lang)
	if sl == nil {
		return nil, nil, nil
	}

	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(sl)
	tree := parser.Parse(nil, src)
	if tree == nil {
		return nil, nil, nil
	}
	defer tree.Close()

	imports, err = extractImports(src, tree.RootNode(), lang, sl)
	if err != nil {
		return nil, nil, err
	}
	symbols, err = extractSymbols(src, tree.RootNode(), lang, sl, path)
	return imports, symbols, err
}

// extractImports runs the language import query and returns deduplicated paths.
func extractImports(src []byte, root *sitter.Node, lang Language, sl *sitter.Language) ([]string, error) {
	qStr := importQueryFor(lang)
	if qStr == "" {
		return nil, nil
	}
	q, err := sitter.NewQuery([]byte(qStr), sl)
	if err != nil {
		// non-fatal — query may not match this grammar version
		return nil, nil
	}
	qc := sitter.NewQueryCursor()
	qc.Exec(q, root)

	seen := make(map[string]bool)
	var imports []string
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			raw := strings.TrimSpace(c.Node.Content(src))
			raw = strings.Trim(raw, "\"'`")
			if raw != "" && !seen[raw] {
				seen[raw] = true
				imports = append(imports, raw)
			}
		}
	}
	return imports, nil
}

// extractSymbols runs the language definition query and returns named symbols.
func extractSymbols(src []byte, root *sitter.Node, lang Language, sl *sitter.Language, filePath string) ([]Symbol, error) {
	qStr := defQueryFor(lang)
	if qStr == "" {
		return nil, nil
	}
	q, err := sitter.NewQuery([]byte(qStr), sl)
	if err != nil {
		return nil, nil
	}
	qc := sitter.NewQueryCursor()
	qc.Exec(q, root)

	var symbols []Symbol
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			capName := q.CaptureNameForId(c.Index)
			name := c.Node.Content(src)
			line := int(c.Node.StartPoint().Row) + 1
			symbols = append(symbols, Symbol{
				Name: name,
				Kind: captureToKind(capName, name, lang),
				File: filePath,
				Line: line,
			})
		}
	}
	return symbols, nil
}

// captureToKind maps a tree-sitter capture name to a Symbol kind constant.
func captureToKind(captureName, symName string, lang Language) string {
	isExportedGo := (lang == LangGo) && len(symName) > 0 && unicode.IsUpper(rune(symName[0]))
	switch captureName {
	case "func", "method":
		if isExportedGo {
			return KindExport
		}
		return KindFunction
	case "class", "impl":
		return KindClass
	case "iface", "trait":
		return KindInterface
	case "type", "struct":
		if isExportedGo {
			return KindExport
		}
		return KindType
	default:
		return KindFunction
	}
}

func importQueryFor(lang Language) string {
	switch lang {
	case LangGo:
		return goImportQuery
	case LangTypeScript:
		return tsImportQuery
	case LangJavaScript:
		return jsImportQuery
	case LangPython:
		return pyImportQuery
	case LangRust:
		return rustImportQuery
	default:
		return ""
	}
}

func defQueryFor(lang Language) string {
	switch lang {
	case LangGo:
		return goDefQuery
	case LangTypeScript:
		return tsDefQuery
	case LangJavaScript:
		return jsDefQuery
	case LangPython:
		return pyDefQuery
	case LangRust:
		return rustDefQuery
	default:
		return ""
	}
}
