package analysis

import (
	"slices"
	"strings"
)

// Graph is one package manager's dependency data: for each installed
// package, the installed packages it directly requires.
type Graph struct {
	Deps  map[string][]string
	Nodes map[string]bool
}

func newGraph() Graph { return Graph{Deps: map[string][]string{}, Nodes: map[string]bool{}} }

func key(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func (g Graph) addNode(pkg string) { g.Nodes[key(pkg)] = true }

func (g Graph) addDep(pkg, dep string) {
	p, d := key(pkg), key(dep)
	if p == d || d == "" || slices.Contains(g.Deps[p], d) {
		return
	}
	g.Deps[p] = append(g.Deps[p], d)
}

// Has reports whether the package is known to this graph.
func (g Graph) Has(pkg string) bool { return g.Nodes[key(pkg)] }

// Dependents returns packages that directly require pkg, and how many more
// require it through those.
func (g Graph) Dependents(pkg string) (direct []string, indirect int) {
	rev := map[string][]string{}
	for p, deps := range g.Deps {
		for _, d := range deps {
			rev[d] = append(rev[d], p)
		}
	}
	target := key(pkg)
	direct = slices.Clone(rev[target])
	slices.Sort(direct)
	seen := map[string]bool{target: true}
	for _, d := range direct {
		seen[d] = true
	}
	queue := slices.Clone(direct)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, p := range rev[cur] {
			if !seen[p] {
				seen[p] = true
				indirect++
				queue = append(queue, p)
			}
		}
	}
	return direct, indirect
}
