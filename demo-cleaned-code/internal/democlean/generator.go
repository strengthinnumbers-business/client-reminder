package democlean

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/strengthinnumbers-business/client-reminder/internal/demolog"
)

const (
	cleanDirName  = "demo-cleaned-code"
	sourceMapName = "demo-source-map.json"
)

type Options struct {
	SourceDir string
	OutputDir string
}

type Entry struct {
	Message        string `json:"message"`
	SourcePath     string `json:"source_path"`
	SourceLine     int    `json:"source_line"`
	SourceFunction string `json:"source_function"`
	TargetMode     string `json:"target_mode"`
	CleanPath      string `json:"clean_path"`
	CleanLine      int    `json:"clean_line"`
	CleanColumn    int    `json:"clean_column"`
}

type removal struct {
	start int
	end   int
}

type pendingEntry struct {
	key          string
	entry        Entry
	sourceRel    string
	targetOffset int
}

func Generate(options Options) error {
	sourceDir, err := filepath.Abs(options.SourceDir)
	if err != nil {
		return err
	}
	outputDir, err := filepath.Abs(options.OutputDir)
	if err != nil {
		return err
	}
	modulePath, err := readModulePath(filepath.Join(sourceDir, "go.mod"))
	if err != nil {
		return err
	}

	if err := os.RemoveAll(outputDir); err != nil {
		return err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte("module demo-cleaned-code\n\ngo 1.26.0\n"), 0o644); err != nil {
		return err
	}

	sourceMap := map[string]Entry{}
	err = filepath.WalkDir(sourceDir, func(path string, dirEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == outputDir || strings.HasPrefix(path, outputDir+string(filepath.Separator)) {
			if dirEntry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if dirEntry.IsDir() {
			switch dirEntry.Name() {
			case ".git", ".gocache", cleanDirName:
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		return generateGoFile(sourceDir, outputDir, modulePath, path, sourceMap)
	})
	if err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(sourceMap, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(filepath.Join(outputDir, sourceMapName), encoded, 0o644)
}

func generateGoFile(sourceDir, outputDir, modulePath, path string, sourceMap map[string]Entry) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(sourceDir, path)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return err
	}

	var removals []removal
	var entries []pendingEntry
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		function := functionName(modulePath, filepath.Dir(rel), fn)
		found, err := collectFromBlock(fset, content, rel, function, fn.Body.List, fn.Body, &removals)
		if err != nil {
			return err
		}
		entries = append(entries, found...)
	}

	cleanContent := applyRemovals(content, removals)
	for _, entry := range entries {
		cleanOffset := adjustedOffset(entry.targetOffset, removals)
		line, column := lineColumn(cleanContent, cleanOffset)
		entry.entry.CleanLine = line
		entry.entry.CleanColumn = column
		if _, exists := sourceMap[entry.key]; exists {
			return fmt.Errorf("source map key collision for %s", entry.key)
		}
		sourceMap[entry.key] = entry.entry
	}

	dest := filepath.Join(outputDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, cleanContent, 0o644)
}

func collectFromBlock(fset *token.FileSet, content []byte, rel, function string, statements []ast.Stmt, enclosing ast.Node, removals *[]removal) ([]pendingEntry, error) {
	var entries []pendingEntry
	for index, stmt := range statements {
		call, mode, message, ok, legacy := demoCall(stmt)
		if legacy {
			pos := fset.Position(stmt.Pos())
			return nil, fmt.Errorf("legacy Demo call at %s:%d:%d", rel, pos.Line, pos.Column)
		}
		if ok {
			target := targetStatement(statements, index, enclosing, mode)
			if target == nil {
				pos := fset.Position(stmt.Pos())
				return nil, fmt.Errorf("could not resolve %s target for demo call at %s:%d:%d", mode, rel, pos.Line, pos.Column)
			}
			sourcePos := fset.Position(stmt.Pos())
			source := demolog.Source{
				Path:     rel,
				Line:     sourcePos.Line,
				Function: function,
				Message:  message,
			}
			cleanPath := filepath.ToSlash(filepath.Join(cleanDirName, rel))
			entries = append(entries, pendingEntry{
				key: demolog.SourceMapKey(source),
				entry: Entry{
					Message:        message,
					SourcePath:     rel,
					SourceLine:     sourcePos.Line,
					SourceFunction: function,
					TargetMode:     mode,
					CleanPath:      cleanPath,
				},
				sourceRel:    rel,
				targetOffset: fset.Position(target.Pos()).Offset,
			})
			*removals = append(*removals, statementRemoval(content, fset.Position(stmt.Pos()).Offset, fset.Position(call.End()).Offset))
			continue
		}

		nested, err := collectNested(fset, content, rel, function, stmt, removals)
		if err != nil {
			return nil, err
		}
		entries = append(entries, nested...)
	}
	return entries, nil
}

func collectNested(fset *token.FileSet, content []byte, rel, function string, stmt ast.Stmt, removals *[]removal) ([]pendingEntry, error) {
	var entries []pendingEntry
	collect := func(statements []ast.Stmt, enclosing ast.Node) error {
		found, err := collectFromBlock(fset, content, rel, function, statements, enclosing, removals)
		if err != nil {
			return err
		}
		entries = append(entries, found...)
		return nil
	}
	switch s := stmt.(type) {
	case *ast.IfStmt:
		if err := collect(s.Body.List, s); err != nil {
			return nil, err
		}
		if s.Else != nil {
			switch elseNode := s.Else.(type) {
			case *ast.BlockStmt:
				if err := collect(elseNode.List, elseNode); err != nil {
					return nil, err
				}
			case *ast.IfStmt:
				found, err := collectNested(fset, content, rel, function, elseNode, removals)
				if err != nil {
					return nil, err
				}
				entries = append(entries, found...)
			}
		}
	case *ast.ForStmt:
		if err := collect(s.Body.List, s); err != nil {
			return nil, err
		}
	case *ast.RangeStmt:
		if err := collect(s.Body.List, s); err != nil {
			return nil, err
		}
	case *ast.SwitchStmt:
		for _, item := range s.Body.List {
			clause := item.(*ast.CaseClause)
			if err := collect(clause.Body, s); err != nil {
				return nil, err
			}
		}
	case *ast.TypeSwitchStmt:
		for _, item := range s.Body.List {
			clause := item.(*ast.CaseClause)
			if err := collect(clause.Body, s); err != nil {
				return nil, err
			}
		}
	case *ast.SelectStmt:
		for _, item := range s.Body.List {
			clause := item.(*ast.CommClause)
			if err := collect(clause.Body, s); err != nil {
				return nil, err
			}
		}
	}
	return entries, nil
}

func demoCall(stmt ast.Stmt) (*ast.CallExpr, string, string, bool, bool) {
	exprStmt, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return nil, "", "", false, false
	}
	call, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return nil, "", "", false, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, "", "", false, false
	}
	mode, ok := map[string]string{
		"DemoBelow":       "below",
		"DemoAbove":       "above",
		"DemoSurrounding": "surrounding",
	}[selector.Sel.Name]
	if selector.Sel.Name == "Demo" {
		return call, "", "", false, true
	}
	if !ok {
		return nil, "", "", false, false
	}
	if len(call.Args) == 0 {
		return nil, "", "", false, false
	}
	messageLiteral, ok := call.Args[0].(*ast.BasicLit)
	if !ok || messageLiteral.Kind != token.STRING {
		return nil, "", "", false, false
	}
	message, err := strconv.Unquote(messageLiteral.Value)
	if err != nil {
		return nil, "", "", false, false
	}
	return call, mode, message, true, false
}

func targetStatement(statements []ast.Stmt, index int, enclosing ast.Node, mode string) ast.Node {
	switch mode {
	case "below":
		for i := index + 1; i < len(statements); i++ {
			if _, _, _, ok, legacy := demoCall(statements[i]); !ok && !legacy {
				return statements[i]
			}
		}
	case "above":
		for i := index - 1; i >= 0; i-- {
			if _, _, _, ok, legacy := demoCall(statements[i]); !ok && !legacy {
				return statements[i]
			}
		}
	case "surrounding":
		return enclosing
	}
	return nil
}

func statementRemoval(content []byte, start, end int) removal {
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	if end < len(content) && content[end] == '\r' {
		end++
	}
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return removal{start: start, end: end}
}

func applyRemovals(content []byte, removals []removal) []byte {
	if len(removals) == 0 {
		return bytes.Clone(content)
	}
	sort.Slice(removals, func(i, j int) bool {
		return removals[i].start < removals[j].start
	})
	var clean bytes.Buffer
	position := 0
	for _, removal := range removals {
		if removal.start > position {
			clean.Write(content[position:removal.start])
		}
		position = removal.end
	}
	clean.Write(content[position:])
	return clean.Bytes()
}

func adjustedOffset(offset int, removals []removal) int {
	adjusted := offset
	for _, removal := range removals {
		if removal.end <= offset {
			adjusted -= removal.end - removal.start
		}
	}
	return adjusted
}

func lineColumn(content []byte, offset int) (int, int) {
	line := 1
	column := 1
	for i := 0; i < len(content) && i < offset; i++ {
		if content[i] == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return line, column
}

func functionName(modulePath, relDir string, fn *ast.FuncDecl) string {
	importPath := modulePath
	if relDir != "." && relDir != "" {
		importPath += "/" + filepath.ToSlash(relDir)
	}
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return importPath + "." + fn.Name.Name
	}
	return importPath + "." + receiverName(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

func receiverName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return "(*" + receiverName(typed.X) + ")"
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	default:
		return "unknown"
	}
}

func readModulePath(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", errors.New("go.mod missing module declaration")
}
