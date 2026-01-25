// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package index provides codebase indexing for fast symbol search.
package index

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"unicode"
)

// =============================================================================
// PARSER INTERFACE
// =============================================================================

// Symbol represents a code symbol (function, class, type, etc.)
type Symbol struct {
	Name       string
	Type       SymbolType
	Line       int
	EndLine    int
	Signature  string
	Doc        string
	Parent     string     // Parent symbol (for methods, nested functions)
	Visibility Visibility
}

// Import represents an import statement
type Import struct {
	Path  string
	Alias string
	Line  int
}

// Parser is the interface for language-specific parsers
type Parser interface {
	// Parse parses source code and extracts symbols
	Parse(content string, filePath string) ([]Symbol, []Import, error)
}

// =============================================================================
// GO PARSER
// =============================================================================

// GoParser parses Go source files
type GoParser struct{}

// Parse implements Parser for Go files
func (p *GoParser) Parse(content string, filePath string) ([]Symbol, []Import, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		// Return the actual error instead of silently ignoring it
		return nil, nil, err
	}

	var symbols []Symbol
	var imports []Import

	// Extract package name
	if f.Name != nil {
		symbols = append(symbols, Symbol{
			Name:       f.Name.Name,
			Type:       SymbolPackage,
			Line:       fset.Position(f.Name.Pos()).Line,
			Signature:  "package " + f.Name.Name,
			Visibility: VisibilityPublic,
		})
	}

	// Extract imports
	for _, imp := range f.Imports {
		impPath := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		imports = append(imports, Import{
			Path:  impPath,
			Alias: alias,
			Line:  fset.Position(imp.Pos()).Line,
		})
	}

	// Walk AST to extract symbols
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			// Function or method
			sym := Symbol{
				Name:       node.Name.Name,
				Type:       SymbolFunction,
				Line:       fset.Position(node.Pos()).Line,
				EndLine:    fset.Position(node.End()).Line,
				Signature:  p.extractFuncSignature(node),
				Doc:        p.extractDoc(node.Doc),
				Visibility: p.getVisibility(node.Name.Name),
			}

			// Check if it's a method
			if node.Recv != nil && len(node.Recv.List) > 0 {
				sym.Type = SymbolMethod
				// Extract receiver type as parent
				if recv := node.Recv.List[0].Type; recv != nil {
					sym.Parent = p.extractTypeName(recv)
				}
			}

			symbols = append(symbols, sym)

		case *ast.GenDecl:
			// Type, const, var declarations
			for _, spec := range node.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					// Type declaration
					sym := Symbol{
						Name:       s.Name.Name,
						Line:       fset.Position(s.Pos()).Line,
						EndLine:    fset.Position(s.End()).Line,
						Doc:        p.extractDoc(node.Doc),
						Visibility: p.getVisibility(s.Name.Name),
					}

					// Determine type kind
					switch s.Type.(type) {
					case *ast.StructType:
						sym.Type = SymbolStruct
						sym.Signature = "type " + s.Name.Name + " struct"
					case *ast.InterfaceType:
						sym.Type = SymbolInterface
						sym.Signature = "type " + s.Name.Name + " interface"
					default:
						sym.Type = SymbolType_
						sym.Signature = "type " + s.Name.Name
					}

					symbols = append(symbols, sym)

				case *ast.ValueSpec:
					// Const or var declaration
					symType := SymbolVariable
					if node.Tok == token.CONST {
						symType = SymbolConst
					}

					for _, name := range s.Names {
						sym := Symbol{
							Name:       name.Name,
							Type:       symType,
							Line:       fset.Position(name.Pos()).Line,
							Doc:        p.extractDoc(node.Doc),
							Visibility: p.getVisibility(name.Name),
						}
						symbols = append(symbols, sym)
					}
				}
			}
		}
		return true
	})

	return symbols, imports, nil
}

// extractFuncSignature extracts a function signature
func (p *GoParser) extractFuncSignature(node *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")

	// Add receiver for methods
	if node.Recv != nil && len(node.Recv.List) > 0 {
		sb.WriteString("(")
		recv := node.Recv.List[0]
		if len(recv.Names) > 0 {
			sb.WriteString(recv.Names[0].Name)
			sb.WriteString(" ")
		}
		sb.WriteString(p.extractTypeName(recv.Type))
		sb.WriteString(") ")
	}

	sb.WriteString(node.Name.Name)
	sb.WriteString("(...)")

	// Add return type if present
	if node.Type.Results != nil && len(node.Type.Results.List) > 0 {
		sb.WriteString(" ")
		if len(node.Type.Results.List) == 1 && len(node.Type.Results.List[0].Names) == 0 {
			sb.WriteString(p.extractTypeName(node.Type.Results.List[0].Type))
		} else {
			sb.WriteString("(...)")
		}
	}

	return sb.String()
}

// extractTypeName extracts type name from an expression
func (p *GoParser) extractTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + p.extractTypeName(t.X)
	case *ast.SelectorExpr:
		return p.extractTypeName(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + p.extractTypeName(t.Elt)
	default:
		return "unknown"
	}
}

// extractDoc extracts documentation from comment group
func (p *GoParser) extractDoc(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	return strings.TrimSpace(cg.Text())
}

// getVisibility determines if a name is exported
func (p *GoParser) getVisibility(name string) Visibility {
	if len(name) == 0 {
		return VisibilityPrivate
	}
	if unicode.IsUpper(rune(name[0])) {
		return VisibilityExported
	}
	return VisibilityPrivate
}

// =============================================================================
// JAVASCRIPT/TYPESCRIPT PARSER
// =============================================================================

// JSParser parses JavaScript/TypeScript files using regex-based extraction
type JSParser struct{}

// Parse implements Parser for JS/TS files
func (p *JSParser) Parse(content string, filePath string) ([]Symbol, []Import, error) {
	var symbols []Symbol
	var imports []Import

	lines := strings.Split(content, "\n")

	// Regex patterns
	funcPattern := regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(`)
	classPattern := regexp.MustCompile(`^\s*(?:export\s+)?class\s+(\w+)`)
	constPattern := regexp.MustCompile(`^\s*(?:export\s+)?const\s+(\w+)\s*=`)
	arrowFuncPattern := regexp.MustCompile(`^\s*(?:export\s+)?const\s+(\w+)\s*=\s*(?:async\s*)?\([^)]*\)\s*=>`)
	importPattern := regexp.MustCompile(`^import\s+(?:.*?from\s+)?['"]([^'"]+)['"]`)

	for i, line := range lines {
		lineNum := i + 1

		// Extract imports
		if matches := importPattern.FindStringSubmatch(line); matches != nil {
			imports = append(imports, Import{
				Path: matches[1],
				Line: lineNum,
			})
		}

		// Extract functions
		if matches := funcPattern.FindStringSubmatch(line); matches != nil {
			symbols = append(symbols, Symbol{
				Name:       matches[1],
				Type:       SymbolFunction,
				Line:       lineNum,
				Signature:  "function " + matches[1] + "(...)",
				Visibility: p.getJSVisibility(line),
			})
		}

		// Extract classes
		if matches := classPattern.FindStringSubmatch(line); matches != nil {
			symbols = append(symbols, Symbol{
				Name:       matches[1],
				Type:       SymbolClass,
				Line:       lineNum,
				Signature:  "class " + matches[1],
				Visibility: p.getJSVisibility(line),
			})
		}

		// Extract arrow functions
		if matches := arrowFuncPattern.FindStringSubmatch(line); matches != nil {
			symbols = append(symbols, Symbol{
				Name:       matches[1],
				Type:       SymbolFunction,
				Line:       lineNum,
				Signature:  "const " + matches[1] + " = (...) =>",
				Visibility: p.getJSVisibility(line),
			})
		}

		// Extract const declarations
		if matches := constPattern.FindStringSubmatch(line); matches != nil {
			// Skip if already captured as arrow function
			if !arrowFuncPattern.MatchString(line) {
				symbols = append(symbols, Symbol{
					Name:       matches[1],
					Type:       SymbolConst,
					Line:       lineNum,
					Visibility: p.getJSVisibility(line),
				})
			}
		}
	}

	return symbols, imports, nil
}

// getJSVisibility checks if a symbol is exported
func (p *JSParser) getJSVisibility(line string) Visibility {
	if strings.Contains(line, "export") {
		return VisibilityExported
	}
	return VisibilityPrivate
}

// =============================================================================
// PYTHON PARSER
// =============================================================================

// PythonParser parses Python files using regex-based extraction
type PythonParser struct{}

// Parse implements Parser for Python files
func (p *PythonParser) Parse(content string, filePath string) ([]Symbol, []Import, error) {
	var symbols []Symbol
	var imports []Import

	lines := strings.Split(content, "\n")

	// Regex patterns
	funcPattern := regexp.MustCompile(`^(\s*)def\s+(\w+)\s*\(`)
	classPattern := regexp.MustCompile(`^(\s*)class\s+(\w+)`)
	importPattern := regexp.MustCompile(`^(?:from\s+(\S+)\s+)?import\s+(.+)`)

	var currentClass string
	var currentIndent int

	for i, line := range lines {
		lineNum := i + 1

		// Extract imports
		if matches := importPattern.FindStringSubmatch(line); matches != nil {
			importPath := matches[1]
			if importPath == "" {
				importPath = strings.TrimSpace(matches[2])
			}
			imports = append(imports, Import{
				Path: importPath,
				Line: lineNum,
			})
		}

		// Extract classes
		if matches := classPattern.FindStringSubmatch(line); matches != nil {
			indent := len(matches[1])
			currentClass = matches[2]
			currentIndent = indent

			symbols = append(symbols, Symbol{
				Name:       currentClass,
				Type:       SymbolClass,
				Line:       lineNum,
				Signature:  "class " + currentClass,
				Visibility: p.getPythonVisibility(currentClass),
			})
		}

		// Extract functions/methods
		if matches := funcPattern.FindStringSubmatch(line); matches != nil {
			indent := len(matches[1])
			funcName := matches[2]

			sym := Symbol{
				Name:       funcName,
				Type:       SymbolFunction,
				Line:       lineNum,
				Signature:  "def " + funcName + "(...)",
				Visibility: p.getPythonVisibility(funcName),
			}

			// Check if it's a method (inside a class)
			if currentClass != "" && indent > currentIndent {
				sym.Type = SymbolMethod
				sym.Parent = currentClass
			} else if indent == 0 {
				// Reset current class only if we're back at module level (indent == 0)
				currentClass = ""
			}

			symbols = append(symbols, sym)
		}

		// Reset current class when we encounter a new class at the same or lower indentation
		if matches := classPattern.FindStringSubmatch(line); matches != nil {
			indent := len(matches[1])
			if indent <= currentIndent && currentClass != "" {
				currentClass = ""
			}
		}
	}

	return symbols, imports, nil
}

// getPythonVisibility checks Python naming conventions
func (p *PythonParser) getPythonVisibility(name string) Visibility {
	if strings.HasPrefix(name, "_") {
		return VisibilityPrivate
	}
	return VisibilityPublic
}
