// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

func main() {
	if len(os.Args) != 5 {
		log.Fatal("Insufficient number of arguments. Need: root filename package functionname")
	}

	root, filename, packageName, fnName := os.Args[1], os.Args[2], os.Args[3], os.Args[4]

	goPackageName := os.Getenv("GOPACKAGE")

	fmt.Printf("generating imports for services in %s\n", goPackageName)

	// Generate the imports by walking the provided directory
	imports := generateImports(root, packageName, fnName)

	buf := &bytes.Buffer{}

	// Create the importing template
	if err := importerTemplate.Execute(buf, struct {
		Package string
		Imports []string
	}{
		Package: goPackageName,
		Imports: imports,
	}); err != nil {
		log.Fatalf("execute templae failed: %v\n", err)
	}

	// Format the importing file
	bs, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatalf("format source failed: %v\n", err)
	}

	// Write it out the filename
	if err := ioutil.WriteFile(filename, bs, 0644); err != nil {
		log.Fatalf("save file failed: %v\n", err)
	}
}

// generateImports walks from the root looking for go files that register a service
func generateImports(root, packageName, fnName string) []string {
	pkgNamesMap := map[string]bool{}
	quotedPackageName := fmt.Sprintf("%q", packageName)
	defaultAlias := packageName[strings.LastIndex(packageName, "/")+1:]

	err := filepath.Walk(root, func(walkpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// skip files we'll deal with them later
			return nil
		}

		// These directories do not contain useful files
		if strings.HasPrefix(walkpath, root+"/.") ||
			strings.HasPrefix(walkpath, root+"/integrations") ||
			strings.HasPrefix(walkpath, root+"/node_modules") ||
			strings.HasPrefix(walkpath, root+"/web_src") ||
			strings.HasPrefix(walkpath, root+"/vendor") {
			return filepath.SkipDir
		}

		// use build to import the dir
		pkg, err := build.ImportDir(walkpath, build.IgnoreVendor)
		if err != nil {
			if _, ok := err.(*build.NoGoError); ok {
				return nil
			}
			return fmt.Errorf("%s %v", walkpath, err)
		}

		// Now only include dirs that import our important package
		hasImport := false
		for _, importName := range pkg.Imports {
			if importName == packageName {
				hasImport = true
				break
			}
		}
		if !hasImport {
			return nil
		}

		// Walk the go files that are in the directory to see if they have the important service call
		serviceCall := false
		for _, filename := range pkg.GoFiles {
			if strings.HasSuffix(filename, "_test.go") {
				continue
			}
			has, err := hasServiceCall(walkpath+"/"+filename, quotedPackageName, defaultAlias, fnName)
			if err != nil {
				return fmt.Errorf("detect file %s failed: %v", walkpath+"/"+filename, err)
			}
			if has {
				serviceCall = true
				break
			}
		}

		if serviceCall {
			pkgNamesMap[path.Clean("code.gitea.io/gitea/"+path.Clean("/"+filepath.ToSlash(walkpath)))] = true
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	pkgNames := make([]string, 0, len(pkgNamesMap))
	for key := range pkgNamesMap {
		pkgNames = append(pkgNames, key)
	}
	return pkgNames
}

func hasServiceCall(filename, quotedPackageName, defaultAlias, fnName string) (bool, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.DeclarationErrors)
	if err != nil {
		return false, err
	}
	hasServiceCall := false

	alias := ""
	for _, importSpec := range file.Imports {
		if importSpec.Path.Value == quotedPackageName {
			if importSpec.Name == nil {
				alias = defaultAlias
			} else {
				alias = importSpec.Name.Name
			}
			break
		}
	}
	if alias == "" || alias == "_" {
		return false, nil
	}

	visitor := &serviceVisitor{
		alias:  alias,
		fnName: fnName,
	}
	ast.Walk(visitor, file)
	if visitor.hasServiceCall {
		hasServiceCall = true
	}

	return hasServiceCall, nil
}

type serviceVisitor struct {
	alias          string
	fnName         string
	hasServiceCall bool
}

func (v *serviceVisitor) Visit(node ast.Node) ast.Visitor {
	if !isCall(node, v.alias, v.fnName) {
		return v
	}

	v.hasServiceCall = true
	return nil
}

func isCall(node ast.Node, alias, fn string) bool {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	aliasId, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	return aliasId.Name == alias && sel.Sel.Name == fn
}

var importerTemplate = template.Must(template.New("").Parse(`// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Code generated by go generate; DO NOT EDIT.
// This file was generated by generate-services.go

package {{.Package}}

import (
	{{range .Imports}}_ "{{.}}"
{{end}}
)
`))
