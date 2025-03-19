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

package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func GenerateTypeRegistration(rootFolder *string) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		if err := os.Chdir(*rootFolder); err != nil {
			fmt.Println("Error changing directory:", err)
			os.Exit(1)
		}

		// modName, err := getModuleName()
		// if err != nil {
		// 	fmt.Println("Error reading go.mod:", err)
		// 	os.Exit(1)
		// }

		// res, err := walkDir(".")
		// if err != nil {
		// 	fmt.Println("Error walking directory:", err)
		// 	os.Exit(1)
		// }

		// fmt.Println(res)
		moduleName, err := getModuleName()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		functions, err := walkDir(".", moduleName)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		err = generateRootFile(moduleName, functions)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
	}
}

// // FunctionCallInfo stores function details
// type FunctionCallInfo struct {
// 	// Name of the component resource function.
// 	FunctionName string
// 	// Name of the struct that contains the resource args.
// 	ResourceArgsStructName string
// 	// Name of the backing struct for the resource.
// 	ResourceStructName string
// 	// Module name of the resource, obtain through the Go module name.
// 	ResourceModuleName string
// }

// // findRegisterComponentResourceFuncs parses a Go file to find functions that call ctx.RegisterComponentResource.
// // We can use this to scafolld the generated code for the provider.
// func findRegisterComponentResourceFuncs(filename string) ([]FunctionCallInfo, error) {
// 	src, err := os.ReadFile(filename)
// 	if err != nil {
// 		return nil, err
// 	}

// 	fset := token.NewFileSet()
// 	node, err := parser.ParseFile(fset, filename, src, parser.AllErrors)
// 	if err != nil {
// 		return nil, err
// 	}

// 	var functionCalls []FunctionCallInfo

// 	// Inspect AST to find function declarations
// 	ast.Inspect(node, func(n ast.Node) bool {
// 		if fn, ok := n.(*ast.FuncDecl); ok {
// 			var firstReturnType string
// 			if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
// 				firstReturnType = extractType(fn.Type.Results.List[0].Type)
// 				firstReturnType = strings.TrimPrefix(firstReturnType, "*")
// 			}

// 			// Ensure the function has at least 3 parameters
// 			if fn.Type.Params == nil || len(fn.Type.Params.List) < 3 {
// 				return true
// 			}

// 			thirdParam := fn.Type.Params.List[2]         // Get the third parameter
// 			thirdArgType := extractType(thirdParam.Type) // Extract type

// 			// Inspect function body for RegisterComponentResource calls
// 			ast.Inspect(fn.Body, func(nn ast.Node) bool {
// 				if callExpr, ok := nn.(*ast.CallExpr); ok {
// 					if sel, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
// 						if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "ctx" && sel.Sel.Name == "RegisterComponentResource" {
// 							functionCalls = append(functionCalls, FunctionCallInfo{
// 								FunctionName:           fn.Name.Name,
// 								ResourceArgsStructName: thirdArgType,
// 								ResourceStructName:     firstReturnType,
// 							})
// 							return false // No need to inspect further in this function
// 						}
// 					}
// 				}
// 				return true
// 			})
// 		}
// 		return true
// 	})

// 	return functionCalls, nil
// }

// // extractType extracts a readable string representation of a Go type.
// func extractType(expr ast.Expr) string {
// 	switch t := expr.(type) {
// 	case *ast.Ident:
// 		return t.Name
// 	case *ast.StarExpr:
// 		return "*" + extractType(t.X) // Handle pointer types
// 	case *ast.SelectorExpr:
// 		return fmt.Sprintf("%s.%s", extractType(t.X), t.Sel.Name) // Handle package-qualified types
// 	case *ast.ArrayType:
// 		return "[]" + extractType(t.Elt) // Handle slices
// 	case *ast.MapType:
// 		return fmt.Sprintf("map[%s]%s", extractType(t.Key), extractType(t.Value)) // Handle maps
// 	case *ast.FuncType:
// 		return "func(...)"
// 	default:
// 		return "unknown"
// 	}
// }

// // walkDir recursively scans `.go` files and applies `findRegisterComponentResourceFuncs`
// func walkDir(root string) ([]FunctionCallInfo, error) {
// 	var results []FunctionCallInfo

// 	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			return err
// 		}
// 		if filepath.Ext(path) == ".go" {
// 			funcs, err := findRegisterComponentResourceFuncs(path)
// 			if err != nil {
// 				return err
// 			}

// 			results = append(results, funcs...)
// 		}
// 		return nil
// 	})

// 	return results, err
// }

// // getModuleName reads the go.mod file and extracts the module name.
// func getModuleName() (string, error) {
// 	data, err := os.ReadFile("go.mod")
// 	if err != nil {
// 		return "", err
// 	}
// 	for _, line := range strings.Split(string(data), "\n") {
// 		if strings.HasPrefix(line, "module ") {
// 			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
// 		}
// 	}
// 	return "", fmt.Errorf("module not found in go.mod")
// }

// // getPackageName returns the package name for a given file path.
// func getPackagePaths(rootDir string) (map[string]string, error) {
// 	packages := make(map[string]string)

// 	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			return err
// 		}
// 		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
// 			fset := token.NewFileSet()
// 			node, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
// 			if err != nil {
// 				return err
// 			}
// 			dir := filepath.Dir(path)
// 			packages[dir] = node.Name.Name
// 		}
// 		return nil
// 	})
// 	return packages, err
// }

// // generateImports generates import statements for the given module name and packages.
// func generateImports(moduleName string, packages map[string]string) []string {
// 	imports := []string{}
// 	for dir, pkg := range packages {
// 		relPath, _ := filepath.Rel(".", dir)
// 		importPath := moduleName + "/" + filepath.ToSlash(relPath)
// 		imports = append(imports, fmt.Sprintf("%s \"%s\"", pkg, importPath))
// 	}
// 	return imports
// }

// ComponentInfo stores details about functions calling RegisterComponentResource
type ComponentInfo struct {
	FunctionName           string
	ResourceArgsStructName string
	ResourceStructName     string
	PackagePath            string
}

// getModuleName extracts the Go module name from go.mod
func getModuleName() (string, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module not found in go.mod")
}

// findRegisterComponentResourceFuncs scans a Go file for functions calling ctx.RegisterComponentResource
func findRegisterComponentResourceFuncs(filename string, moduleName string) ([]ComponentInfo, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, src, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	var functionCalls []ComponentInfo
	relPath, _ := filepath.Rel(".", filepath.Dir(filename))
	packagePath := moduleName
	if relPath != "." {
		packagePath += "/" + filepath.ToSlash(relPath)
	}

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
							functionCalls = append(functionCalls, ComponentInfo{
								FunctionName:           fn.Name.Name,
								ResourceArgsStructName: thirdArgType,
								ResourceStructName:     firstReturnType,
								PackagePath:            packagePath,
							})
							return false // Stop further inspection in this function
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

// extractType extracts a readable string representation of a Go type
func extractType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + extractType(t.X)
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", extractType(t.X), t.Sel.Name)
	case *ast.ArrayType:
		return "[]" + extractType(t.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", extractType(t.Key), extractType(t.Value))
	case *ast.FuncType:
		return "func(...)"
	default:
		return "unknown"
	}
}

// walkDir recursively scans `.go` files and applies `findRegisterComponentResourceFuncs`
func walkDir(root string, moduleName string) ([]ComponentInfo, error) {
	var results []ComponentInfo

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go") && filepath.Base(path) != "pulumi_generated.types.go " {
			funcs, err := findRegisterComponentResourceFuncs(path, moduleName)
			if err != nil {
				return err
			}
			results = append(results, funcs...)
		}
		return nil
	})

	return results, err
}

// generateRootFile creates a Go file with import statements and function references
func generateRootFile(moduleName string, componentResources []ComponentInfo) error {
	var buffer bytes.Buffer
	buffer.WriteString(`package main

import(
`)

	// Generate unique imports, excluding root package
	imports := make(map[string]string)
	for _, fn := range componentResources {
		if fn.PackagePath == moduleName {
			continue // Skip import for root package
		}

		alias := filepath.Base(fn.PackagePath)
		if existing, found := imports[alias]; found && existing != fn.PackagePath {
			alias = alias + "1" // Avoid conflicts
		}
		imports[alias] = fn.PackagePath
	}
	for alias, path := range imports {
		buffer.WriteString(fmt.Sprintf("\t%s \"%s\"\n", alias, path))
	}
	buffer.WriteString(`
	"github.com/pulumi/pulumi-go-provider/component"
	"github.com/pulumi/pulumi-go-provider/infer"
`)
	buffer.WriteString(")\n\n")

	// Generate function wrappers
	buffer.WriteString("func init() {\n")
	for _, fn := range componentResources {
		var call string
		if fn.PackagePath == moduleName {
			call = fmt.Sprintf(`component.RegisterType(infer.ComponentProviderResource(
		infer.ComponentFn[%s, *%s](%s)))`, fn.ResourceArgsStructName, fn.ResourceStructName, fn.FunctionName)
		} else {
			modImport := filepath.Base(fn.PackagePath)
			call = fmt.Sprintf(`component.RegisterType(infer.ComponentProviderResource(
		infer.ComponentFn[%s.%s, *%s.%s](%s.%s)))`, modImport, fn.ResourceArgsStructName, modImport, fn.ResourceStructName, modImport, fn.FunctionName)
		}
		buffer.WriteString("\t" + call + "\n")
	}
	buffer.WriteString("}\n")

	// Write to file
	return os.WriteFile("pulumi_generated.types.go", buffer.Bytes(), 0644)
}
