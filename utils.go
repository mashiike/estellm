package estellm

import (
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"text/template"
)

func ptr[T any](v T) *T {
	return &v
}

func findDAGs(dependents map[string][]string) []map[string][]string {
	visited := make(map[string]bool)
	var result []map[string][]string

	for node := range dependents {
		if !visited[node] {
			subGraph := exploreDAG(node, dependents)
			result = append(result, subGraph)
			for node := range subGraph {
				visited[node] = true
			}
		}
	}
	slices.SortFunc(result, func(a, b map[string][]string) int {
		return cmp.Or(
			cmp.Compare(-1*len(a), -1*len(b)),
		)
	})
	return result
}

func pickupDAG(targetNode string, dependents map[string][]string) (map[string][]string, bool) {
	dags := findDAGs(dependents)
	if len(dags) == 0 {
		return nil, false
	}
	for _, dag := range dags {
		if _, ok := dag[targetNode]; ok {
			return dag, true
		}
	}
	return nil, false
}

func exploreDAG(target string, dependents map[string][]string) map[string][]string {
	subGraph := extractDownstreamSubgraph(reverseDependency(dependents), target)
	sources := []string{}
	for node, neighbors := range subGraph {
		if len(neighbors) == 0 {
			sources = append(sources, node)
		}
	}
	if len(sources) == 0 {
		// no source node found, maybe cycle graph
		return dependents
	}
	graph := make(map[string][]string)
	for _, source := range sources {
		subGraph := extractDownstreamSubgraph(dependents, source)
		for node, neighbors := range subGraph {
			graph[node] = neighbors
		}
	}
	return graph
}

// extruct subgraph from start node to all reachable nodes
func extractDownstreamSubgraph(graph map[string][]string, start string) map[string][]string {
	if start == "" {
		return graph
	}

	subGraph := make(map[string][]string)
	visited := make(map[string]bool)
	var dfs func(string)
	dfs = func(node string) {
		if visited[node] {
			return
		}
		visited[node] = true
		subGraph[node] = graph[node]

		for _, neighbor := range graph[node] {
			dfs(neighbor)
		}
	}
	dfs(start)
	return subGraph
}

func extractUpstreamSubgraph(graph map[string][]string, start string) map[string][]string {
	subGraph := extractDownstreamSubgraph(reverseDependency(graph), start)
	return reverseDependency(subGraph)
}

func topologicalSort(graph map[string][]string) ([][]string, error) {
	inDegree := make(map[string]int)
	for node, deps := range graph {
		if _, exists := inDegree[node]; !exists {
			inDegree[node] = 0
		}
		for _, dep := range deps {
			inDegree[dep]++
		}
	}

	var result [][]string
	var zeroNodes []string
	for node, degree := range inDegree {
		if degree == 0 {
			zeroNodes = append(zeroNodes, node)
			slices.Sort(zeroNodes)
		}
	}

	for len(zeroNodes) > 0 {
		result = append(result, zeroNodes)
		var nextZero []string

		for _, node := range zeroNodes {
			for _, neighbor := range graph[node] {
				inDegree[neighbor]--
				if inDegree[neighbor] == 0 {
					nextZero = append(nextZero, neighbor)
				}
			}
		}
		zeroNodes = nextZero
	}

	for _, degree := range inDegree {
		if degree != 0 {
			return nil, errors.New("cycle detected")
		}
	}

	return result, nil
}

func findSinkNodes(graph map[string][]string) []string {
	sinkNodes := []string{}
	var walk func(string, string)
	walk = func(start string, node string) {
		if len(graph[node]) == 0 {
			sinkNodes = append(sinkNodes, node)
		}
		for _, neighbor := range graph[node] {
			if start != neighbor {
				walk(start, neighbor)
			}
		}
	}
	for node := range graph {
		walk(node, node)
	}
	slices.Sort(sinkNodes)
	return slices.Compact(sinkNodes)
}

func reverseDependency(dependents map[string][]string) map[string][]string {
	dependsOn := make(map[string][]string)
	for name, deps := range dependents {
		if _, exists := dependsOn[name]; !exists {
			dependsOn[name] = []string{}
		}
		for _, dep := range deps {
			dependsOn[dep] = append(dependsOn[dep], name)
		}
	}
	return dependsOn
}

func findSourceNodes(graph map[string][]string) []string {
	reversed := reverseDependency(graph)
	for node, deps := range graph {
		if len(deps) == 0 {
			delete(reversed, node)
		}
	}
	return findSinkNodes(reversed)
}

func mergeFuncMaps(funcMaps map[string]template.FuncMap) (template.FuncMap, error) {
	merged := template.FuncMap{}

	for _, funcMap := range funcMaps {
		for name, fn := range funcMap {
			// check if function name conflict
			if existingFn, exists := merged[name]; exists {
				if !isSameSignature(existingFn, fn) {
					return nil, fmt.Errorf("function name conflict: %s has different signatures", name)
				}
			}
			merged[name] = fn
		}
	}

	return merged, nil
}

func isSameSignature(fn1, fn2 interface{}) bool {
	t1 := reflect.TypeOf(fn1)
	t2 := reflect.TypeOf(fn2)

	return t1 != nil && t2 != nil && t1.Kind() == reflect.Func && t1 == t2
}

func wildcardMatchs(pattern string, strs []string) ([]string, error) {
	regexPattern := "^" + regexp.QuoteMeta(pattern) + "$"
	regexPattern = strings.ReplaceAll(regexPattern, `\*`, ".*")

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, err
	}
	matched := make([]string, 0, len(strs))
	for _, str := range strs {
		if re.MatchString(str) {
			matched = append(matched, str)
		}
	}
	return matched, nil
}
