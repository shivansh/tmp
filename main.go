// Renames function declarations and calls.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
)

const (
	path   = "/home/shivansh/code/grsync/parser/parser.go"
	before = "diff"      // initial function name
	after  = "translate" // modified function name
)

func main() {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok {
				if id.Name == before {
					fmt.Printf("%s called at: %s\n", before, fset.Position(n.Pos()))
					id.Name = after
				}
			}
		case *ast.FuncDecl:
			if x.Name.Name == before {
				fmt.Printf("%s declared at: %s\n", before, fset.Position(n.Pos()))
				x.Name.Name = after
			}
		}

		return true
	})

	dst, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	printer.Fprint(dst, fset, file)
}
