package main

import (
	"html/template"
	"path/filepath"
	"strings"
)

type ComponentsGraph map[*Component][]*Component

func (graph ComponentsGraph) CyclicComponents(node *Component, visited map[*Component]bool, stack map[*Component]bool, trace *[]*Component) bool {
	visited[node] = true
	stack[node] = true
	*trace = append(*trace, node)

	// Traverse all dependencies of the current component
	for _, component := range graph[node] {
		if !visited[component] {
			if graph.CyclicComponents(component, visited, stack, trace) {
				return true // Cycle found
			}
		} else if stack[component] {
			*trace = append(*trace, component)
			return true // Cycle found (back edge detected)
		}
	}
	stack[node] = false
	*trace = (*trace)[:len(*trace)-1]
	return false
}

// checks if the components graph contains a cycle using
// Depth-First-Search (DFS) algorithm
func (graph ComponentsGraph) FailOnCyclicComponents() {
	visited := make(map[*Component]bool)
	stack := make(map[*Component]bool)
	trace := make([]*Component, 0)

	// Run DFS for every node that hasn't been visited
	for component := range graph {
		if !visited[component] {
			if graph.CyclicComponents(component, visited, stack, &trace) {
				for i, c := range trace {
					printInfo("%*s%s\n", (i+1)*2, "", c.Path)
				}
				printError("error: cyclic reference found\n")
			}
		}
	}
}

type Component struct {
	// @todo: maybe grab the content of the script tag and put it on a
	// .js file and put it on static folder and then load if on app.html
	Tag, Name, Path string

	Content template.HTML

	// this is used in the first pass when we collect the html tags
	// prefixed by `WEBCOMPONENT_PREFIX`. In this phase we do not check
	// for recursive components or cyclic components definition
	WcTags []string

	// this is used in the second pass where we will check of recursive
	// and cyclic components
	Components []*Component
}

func NewComponent(path string) *Component {
	component := &Component{Path: path}

	component.Name = strings.Split(filepath.Base(component.Path), ".")[0]
	component.Tag = strings.Replace(component.Name, "_", "-", -1)

	if !strings.HasPrefix(component.Tag, WEBCOMPONENT_PREFIX) {
		// @todo: improve error message
		printError("error: web component %s must be prefixed with %s", path, WEBCOMPONENT_PREFIX)
	}

	tmpl := loadTemplateFile(component.Path)

	var buf strings.Builder
	uniqueID := GenerateUniqueID()
	if err := tmpl.Execute(&buf, map[string]any{"Tag": component.Tag, "UniqueID": uniqueID}); err != nil {
		printError("error: could build component %s: %v", component.Path, err)
	}
	contentStr := buf.String()

	component.Content = template.HTML(contentStr)
	component.WcTags, _ = collectWebComponentTags(contentStr, false)

	return component
}

func (c *Component) PopulateComponents(components map[string]*Component) {
	c.Components = make([]*Component, 0, len(c.WcTags))
	for _, tag := range c.WcTags {
		component, found := components[tag]
		if !found {
			printError("error: could not build component %s graph because `%s` is not defined\n", c.Path, tag)
		}
		c.Components = append(c.Components, component)
	}
}

func NewComponentsGraph(components map[string]*Component) ComponentsGraph {
	graph := make(ComponentsGraph, 0)
	for _, value := range components {
		graph[value] = value.Components
	}
	return graph
}
