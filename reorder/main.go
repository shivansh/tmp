// reorder reorders the function declarations in a go file such that a call
// expression's position precedes the corresponding function's declaration.
//
// Usage:
//
//		reorder [-svg graph.svg] -path file.go
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

	"github.com/goccy/go-graphviz"
	"golang.org/x/tools/go/ast/astutil"
)

var (
	svg  = flag.String("svg", "", "absolute path of the rendered SVG file")
	path = flag.String("path", "", "absolute path of the file to be modified")
)

func usage() {
	fmt.Fprint(os.Stderr, "usage: reorder [-svg graph.svg] -path file.go")
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *path == "" {
		usage()
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, *path, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	nodes := make(map[string]ast.Node)
	inedge := make(map[string]int)
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			nodes[x.Name.Name] = x
			inedge[x.Name.Name] = 0
		}
		return true
	})

	var parent string
	graph := make(map[string][]string)
	astutil.Apply(
		file,
		func(c *astutil.Cursor) bool { // pre-order
			n := c.Node()
			switch x := n.(type) {
			case *ast.FuncDecl:
				parent = x.Name.Name
			case *ast.CallExpr:
				if y, ok := x.Fun.(*ast.Ident); ok {
					if _, ok := nodes[y.Name]; ok && parent != y.Name && !contains(graph[parent], y.Name) {
						graph[parent] = append(graph[parent], y.Name)
						inedge[y.Name]++
					}
				}
			}
			return true
		},
		nil)

	if *svg != "" {
		render(nodes, graph)
	}

	if !isDAG(graph) {
		log.Fatal("Graph contains a cycle, topological ordering is not possible.")
	}

	// Topologically sort.
	queue := []string{}
	for u, count := range inedge {
		if count == 0 {
			queue = append(queue, u)
		}
	}
	index := 0
	for index < len(queue) {
		u := queue[index]
		index++
		for _, v := range graph[u] {
			inedge[v]--
			if inedge[v] == 0 {
				queue = append(queue, v)
			}
		}
	}

	index = 0
	astutil.Apply(file, nil, func(c *astutil.Cursor) bool {
		n := c.Node()
		switch n.(type) {
		case *ast.FuncDecl:
			node := nodes[queue[index]]
			c.Replace(node)
			// TODO: reorder the associated CommentGroup.
			// c.InsertBefore() doesn't work as current node is the child of *ast.FILE
			// and the node being inserted should be of type *ast.Decl.
			index++
		}
		return true
	})

	dst, err := os.OpenFile(*path, os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	format.Node(dst, fset, file)
}

func contains(s []string, val string) bool {
	for _, v := range s {
		if v == val {
			return true
		}
	}
	return false
}

func render(nodes map[string]ast.Node, graph map[string][]string) {
	g := graphviz.New()
	gr, err := g.Graph()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := gr.Close(); err != nil {
			log.Fatal(err)
		}
		g.Close()
	}()

	for u := range nodes {
		parent, err := gr.CreateNode(u)
		if err != nil {
			log.Fatal(err)
		}
		for _, v := range graph[u] {
			child, err := gr.CreateNode(v)
			if err != nil {
				log.Fatal(err)
			}
			gr.CreateEdge("e", parent, child)
		}
	}

	if err := g.RenderFilename(gr, graphviz.SVG, *svg); err != nil {
		log.Fatal(err)
	}
}

// TODO: optimize
func isDAG(graph map[string][]string) bool {
	for root := range graph {
		visited := make(map[string]struct{})
		queue := []string{}
		queue = append(queue, root)
		for len(queue) > 0 {
			u := queue[0]
			visited[u] = struct{}{}
			queue = queue[1:]
			for _, v := range graph[u] {
				if _, ok := visited[v]; ok {
					return false
				}
				queue = append(queue, v)
			}
		}
	}
	return true
}
