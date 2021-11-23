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
	"log"
	"os"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/goccy/go-graphviz"
)

var (
	svg = flag.String("svg", "", "(optional) path of the rendered SVG file")
)

func usage() {
	fmt.Fprint(os.Stderr, "usage: reorder [-svg graph.svg] file.go\n")
	os.Exit(2)
}

func main() {
	flag.Parse()
	flag.Usage = usage
	args := flag.Args()
	if len(args) != 1 {
		usage()
	}
	path := args[0]

	f, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	file, err := decorator.Parse(f)
	if err != nil {
		log.Fatal(err)
	}

	nodes := make(map[string]dst.Node)
	inedge := make(map[string]int)
	dst.Inspect(file, func(n dst.Node) bool {
		switch x := n.(type) {
		case *dst.FuncDecl:
			nodes[x.Name.Name] = x
			inedge[x.Name.Name] = 0
		}
		return true
	})

	var parent string
	graph := make(map[string][]string)
	dstutil.Apply(
		file,
		func(c *dstutil.Cursor) bool { // pre-order
			n := c.Node()
			switch x := n.(type) {
			case *dst.FuncDecl:
				parent = x.Name.Name
			case *dst.CallExpr:
				if y, ok := x.Fun.(*dst.Ident); ok {
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
	dstutil.Apply(file, nil, func(c *dstutil.Cursor) bool {
		n := c.Node()
		switch n.(type) {
		case *dst.FuncDecl:
			node := nodes[queue[index]]
			c.Replace(node) // https://github.com/golang/go/issues/20744
			index++
		}
		return true
	})

	dest, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	decorator.Fprint(dest, file)
}

func contains(s []string, val string) bool {
	for _, v := range s {
		if v == val {
			return true
		}
	}
	return false
}

func render(nodes map[string]dst.Node, graph map[string][]string) {
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
