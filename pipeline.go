package main

import (
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

var PipelineContinue error = errors.New("continue to next in pipeline")
var PipelineBreak error = errors.New("break pipeline without error")

type PipelineFunc func(page *Page) (*Page, error)

var pipeline = []PipelineFunc{
	customExample,
	read,
	// setData,
	parseTemplate,
	collectHeadAndComponents,
	setLayoutData,
	mergeWithLayout,
	write,
}

func read(page *Page) (*Page, error) {
	content, err := os.ReadFile(page.Path)
	if err != nil {
		return page, fmt.Errorf("error: could not read %s content: %v\n", page.Path, err)
	}
	page.Content = string(content[:])
	return page, PipelineContinue
}

func parseTemplate(page *Page) (*Page, error) {
	tmpl := template.New("").Funcs(page.Funcs)
	tmpl, err := tmpl.Parse(page.Content)
	if err != nil {
		return page, fmt.Errorf("error: could not parse template %s: %v\n", page.Path, err)
	}

	var buf strings.Builder
	page.Data["UniqueID"] = GenerateUniqueID()
	if err := tmpl.Execute(&buf, page.Data); err != nil {
		printError("error: could not compile page %s: %v", page.Path, err)
	}
	page.Content = buf.String()

	return page, PipelineContinue
}

func collectHeadAndComponents(page *Page) (*Page, error) {
	visited := make(map[*Component]bool)
	cmps := make([]*Component, 0)
	deps, headContent := collectWebComponentTags(page.Content, true)

	for _, dep := range deps {
		component, found := page.DefinedComponents[dep]
		if found {
			if !visited[component] {
				dfsCollectComponents(component, page.ComponentsGraph, visited, &cmps)
			}
		} else {
			printError("error: page %s requires component %s which is missing\n", page.Path, dep)
		}
	}

	page.UsedComponents = cmps
	page.Head = headContent

	return page, PipelineContinue
}

func setLayoutData(page *Page) (*Page, error) {
	page.Data["Components"] = page.UsedComponents
	page.Data["Content"] = template.HTML(page.Content)
	page.Data["Head"] = template.HTML(page.Head)

	return page, PipelineContinue
}

func mergeWithLayout(page *Page) (*Page, error) {
	var buf strings.Builder
	appTmpl := loadTemplateFile(page.Layout)
	if err := appTmpl.Execute(&buf, page.Data); err != nil {
		printError("error: could not merge page %s with layout %s: %v", page.Path, page.Layout, err)
	}
	page.Content = buf.String()
	return page, PipelineContinue
}

func write(page *Page) (*Page, error) {
	if _, err := os.Stat(page.Dest); err != nil {
		if os.IsNotExist(err) {
			dir := filepath.Dir(page.Dest)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return page, fmt.Errorf("error: could not create directory %s: %v\n", page.Path, err)
			}
			printInfo("info: created directory %s\n", dir)
		} else {
			return page, fmt.Errorf("error: could not stat path %s: %v\n", page.Path, err)
		}
	}

	if err := os.WriteFile(page.Dest, []byte(page.Content), 0755); err != nil {
		return page, fmt.Errorf("error: could not write file %s: %v\n", page.Path, err)
	}
	return page, PipelineContinue
}
