// rename renames a function in a go file (declarations and corresponding call
// expressions).
//
// Usage:
//
//		rename -path file.go -before foo -after bar
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
)

var (
	path   = flag.String("path", "", "absolute path of file to be modified")
	before = flag.String("before", "", "initial function name")
	after  = flag.String("after", "", "modified function name")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: rename -path file.go -before foo -after bar\n")
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *path == "" || *before == "" || *after == "" {
		usage()
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, *path, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok {
				if id.Name == *before {
					fmt.Printf("%s called at: %s\n", *before, fset.Position(n.Pos()))
					id.Name = *after
				}
			}
		case *ast.FuncDecl:
			if x.Name.Name == *before {
				fmt.Printf("%s declared at: %s\n", *before, fset.Position(n.Pos()))
				x.Name.Name = *after
			}
		}
		return true
	})

	dst, err := os.OpenFile(*path, os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	format.Node(dst, fset, file)
}
