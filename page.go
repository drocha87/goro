package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"
	"strings"
)

type Page struct {
	Path   string // page path
	Dest   string // destination path generally it will be constructed like DIST_BASEPATH/(Path without the prefix PAGES_BASEPATH)
	Layout string // default to ./app.html

	Content string
	Head    string

	ComponentsGraph   ComponentsGraph
	DefinedComponents map[string]*Component
	UsedComponents    []*Component

	Data       map[string]any
	LayoutData map[string]any

	Funcs template.FuncMap
}

func processPages(root string, definedComponents map[string]*Component, graph ComponentsGraph) {
	err := filepath.WalkDir(root, func(path string, info fs.DirEntry, err error) error {
		if err == nil {
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".html") {
				dest := filepath.Clean(fmt.Sprintf("%s/%s", DIST_BASEPATH, strings.TrimPrefix(path, PAGES_BASEPATH)))
				page := &Page{
					Path:              path,
					Dest:              dest,
					Layout:            "./app.html",
					Content:           "",
					ComponentsGraph:   graph,
					DefinedComponents: definedComponents,
					UsedComponents:    make([]*Component, 0),
					Data:              make(map[string]any),
					Funcs:             funcMap,
				}

				for _, fn := range pipeline {
					if page, err := fn(page); err != nil {
						if err == PipelineContinue {
							continue
						}
						if err == PipelineBreak {
							break
						}
						printError("error: could not process page %s: %v\n", page.Path, err)
					}
				}
			}
		}
		return err
	})

	if err != nil {
		printError("error: processing pages: %v\n", err)
	}
}
