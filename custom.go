package main

import (
	"fmt"
	"strings"
	"path/filepath"
	"os"
)

func customExample(page *Page) (*Page, error) {
	if page.Path == "pages/[foo]/index.html" {
		content, err := os.ReadFile(page.Path)
		if err != nil {
			return page, fmt.Errorf("error: could not read %s content: %v\n", page.Path, err)
		}

		paths := []string{"foo", "bar", "baz"}

		for _, path := range paths {
			p := strings.Replace(page.Path, "[foo]", path, 1)
			d := filepath.Clean(fmt.Sprintf("%s/%s", DIST_BASEPATH, strings.TrimPrefix(p, PAGES_BASEPATH)))
			page := &Page{
				Path:              p,
				Dest:              d,
				Layout:            "./app.html",
				Content:           string(content[:]),
				ComponentsGraph:   page.ComponentsGraph,
				DefinedComponents: page.DefinedComponents,
				UsedComponents:    make([]*Component, 0),
				Data:              map[string]any{"Text": fmt.Sprintf("%s: uhuull", p)},
				Funcs:             funcMap,
			}

			// create a custom pipeline without the read command since we are
			// creating the page on the fly.
			pipeline := []PipelineFunc{
				parseTemplate,
				collectHeadAndComponents,
				setLayoutData,
				mergeWithLayout,
				write,
			}

			for _, fn := range pipeline {
				if page, err := fn(page); err != nil {
					if err == PipelineContinue {
						continue
					}
					if err == PipelineBreak {
						break
					}
					printError("error: could not process template page %s: %v\n", page.Path, err)
				}
			}
		}
	} else {
		// if the path is not what we want just continue to next in pipeline
		return page, PipelineContinue
	}
	return page, PipelineBreak
}
