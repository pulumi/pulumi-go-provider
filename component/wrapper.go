// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package component

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
)

type globalRegistry struct {
	InferredComponents []infer.InferredComponent
}

var registry = globalRegistry{
	InferredComponents: []infer.InferredComponent{},
}

// RegisterType registers a type with the global registry.
func RegisterType(ic infer.InferredComponent) {
	registry.InferredComponents = append(registry.InferredComponents, ic)
}

// FunctionCallInfo stores function details
type FunctionCallInfo struct {
	// Name of the component resource function.
	FunctionName string
	// Name of the struct that contains the resource args.
	ResourceArgsStructName string
	// Name of the backing struct for the resource.
	ResourceStructName string
}

// findRegisterComponentResourceFuncs parses a Go file to find functions that call ctx.RegisterComponentResource.
// We can use this to scafolld the generated code for the provider.
func findRegisterComponentResourceFuncs(filename string) ([]FunctionCallInfo, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, src, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	var functionCalls []FunctionCallInfo

	// Inspect AST to find function declarations
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			var firstReturnType string
			if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
				firstReturnType = extractType(fn.Type.Results.List[0].Type)
				firstReturnType = strings.TrimPrefix(firstReturnType, "*")
			}

			// Ensure the function has at least 3 parameters
			if fn.Type.Params == nil || len(fn.Type.Params.List) < 3 {
				return true
			}

			thirdParam := fn.Type.Params.List[2]         // Get the third parameter
			thirdArgType := extractType(thirdParam.Type) // Extract type

			// Inspect function body for RegisterComponentResource calls
			ast.Inspect(fn.Body, func(nn ast.Node) bool {
				if callExpr, ok := nn.(*ast.CallExpr); ok {
					if sel, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
						if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "ctx" && sel.Sel.Name == "RegisterComponentResource" {
							functionCalls = append(functionCalls, FunctionCallInfo{
								FunctionName:           fn.Name.Name,
								ResourceArgsStructName: thirdArgType,
								ResourceStructName:     firstReturnType,
							})
							return false // No need to inspect further in this function
						}
					}
				}
				return true
			})
		}
		return true
	})

	return functionCalls, nil
}

// extractType extracts a readable string representation of a Go type.
func extractType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + extractType(t.X) // Handle pointer types
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", extractType(t.X), t.Sel.Name) // Handle package-qualified types
	case *ast.ArrayType:
		return "[]" + extractType(t.Elt) // Handle slices
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", extractType(t.Key), extractType(t.Value)) // Handle maps
	case *ast.FuncType:
		return "func(...)"
	default:
		return "unknown"
	}
}

// walkDir recursively scans `.go` files and applies `findRegisterComponentResourceFuncs`
func walkDir(root string) ([]FunctionCallInfo, error) {
	var results []FunctionCallInfo

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) == ".go" {
			funcs, err := findRegisterComponentResourceFuncs(path)
			if err != nil {
				return err
			}

			results = append(results, funcs...)
		}
		return nil
	})

	return results, err
}

func ProviderHost() {
	p.RunProvider("go-components", "0.1.0", provider())
}

func provider() p.Provider {
	opt := infer.Options{
		// Components: []infer.InferredComponent{
		// 	infer.Component[*RandomComponent, RandomComponentArgs, *RandomComponentState](),
		// },
		Metadata: schema.Metadata{
			LanguageMap: map[string]any{
				"nodejs": map[string]any{
					"dependencies": map[string]any{
						"@pulumi/random": "^4.16.8",
					},
					"respectSchemaVersion": true,
				},
				"go": map[string]any{
					"generateResourceContainerTypes": true,
					"respectSchemaVersion":           true,
				},
				"python": map[string]any{
					"requires": map[string]any{
						"pulumi":        ">=3.0.0,<4.0.0",
						"pulumi_random": ">=4.0.0,<5.0.0",
					},
					"respectSchemaVersion": true,
				},
				"csharp": map[string]any{
					"packageReferences": map[string]any{
						"Pulumi":        "3.*",
						"Pulumi.Random": "4.*",
					},
					"respectSchemaVersion": true,
				},
			},
		},
	}

	opt.Components = append(opt.Components, registry.InferredComponents...)

	return infer.Provider(opt)
}
