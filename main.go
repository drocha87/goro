package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/net/html"
)

var (
	IS_TAG   = []byte("is")
	HEAD_TAG = []byte("head")
)

var usedIDS map[string]bool

// prints error to stdout and exit
func printError(format string, v ...any) {
	log.Fatalf(format, v...)
}

// prints error to stdout and exit like printError
func printInfo(format string, v ...any) {
	if VERBOSE {
		log.Printf(format, v...)
	}
}

func main() {
	log.SetFlags(0)
	usedIDS = make(map[string]bool)

	// First pass: Traverse the `COMPONENTS_BASEPATH` to generate a `Component`.
	// In this phase, only the `Tag`, `Name`, `Path`, `Content`, and `WcTags` fields are loaded
	// and appended to `components`. The `Components` slice is not fully populated after this call.
	components := processComponents(COMPONENTS_BASEPATH)

	// This is necessary to build the components graph used to check for
	// cyclic or recursive components
	for _, component := range components {
		component.PopulateComponents(components)
	}

	printInfo("info: checking for cyclic or recursive component...\n")

	graph := NewComponentsGraph(components)

	// traverse the graph looking for cycles using Depth-First-Search
	// algorithm, if we found a cycle we exit with a failure message
	graph.FailOnCyclicComponents()

	printInfo("info: done...\n\n")

	createBuildFolder()

	// Note: This must be executed after parseFiles(COMPONENTS_BASEPATH,
	// false) to ensure the component dependency is defined. Running it
	// before will cause failure.
	processPages(PAGES_BASEPATH, components, graph)

	// copy asset files
	err := copyAssetsFolder(ASSETS_BASEPATH, fmt.Sprintf("%s/assets", DIST_BASEPATH))
	if err != nil {
		printInfo("error: could not finish copying %s folder: %v\n", ASSETS_BASEPATH, err)
	}
}

func isUniqueID(id string) bool {
	_, found := usedIDS[id]
	return found == false
}

// generates a unique small identifier.
func GenerateUniqueID() string {
	for {
		randomBytes := make([]byte, 10)
		_, err := rand.Read(randomBytes)
		if err != nil {
			printError("error: could not generate random indentifier: %v\n", err)
		}
		id := base64.RawURLEncoding.EncodeToString(randomBytes)
		if _, found := usedIDS[id]; !found {
			usedIDS[id] = true
			return id
		}
	}
}

func createBuildFolder() {
	err := os.MkdirAll(DIST_BASEPATH, 0755)
	if err != nil {
		printError(err.Error())
	}
}

// map of functions available to be used on templates
var funcMap = template.FuncMap{
	"gen_unique_id": GenerateUniqueID,
}

// this function returns a parsed template injecting funcs defined in funcMap
func loadTemplateFile(path string) *template.Template {
	tmpl := template.New("").Funcs(funcMap)
	content, err := os.ReadFile(path)
	if err != nil {
		printError("error: could not read %s content: %v\n", path, err)
	}
	tmpl, err = tmpl.Parse(string(content[:]))
	if err != nil {
		printError("error: could not parse template %s: %v\n", path, err)
	}
	return tmpl
}

// this function collects all web component tags (HTML tags prefixed
// with WEBCOMPONENT_PREFIX).
func collectWebComponentTags(content string, isPage bool) ([]string, string) {
	tokens := html.NewTokenizer(strings.NewReader(content))
	// collect only distinct web components
	distinctTags := make(map[string]bool, 0)

	var headBuf strings.Builder
	for tt := tokens.Next(); tt != html.ErrorToken; tt = tokens.Next() {
		if tt == html.StartTagToken {
			name, hasAttr := tokens.TagName()

			if reflect.DeepEqual(name, HEAD_TAG) {
				if isPage {
					// Collect the page <head> children and append their raw
					// HTML content to headBuf.  The contents of headBuf will be
					// inserted into the <head> section of app.html.
					// @note: This operation does not perform any validation; it
					// simply retrieves the innerHTML of the <head> and appends
					// it to the headBuf.
					for tt := tokens.Next(); tt != html.ErrorToken; tt = tokens.Next() {
						name, _ := tokens.TagName()
						if tt == html.EndTagToken && reflect.DeepEqual(name, HEAD_TAG) {
							break
						}
						headBuf.WriteString(string(tokens.Raw()))
					}
				} else {
					// @fixme: <head> tags is not allowed on components for now!
					printError("error: <head> tag is not allowed on component!")
				}
				continue
			}

			tag := string(name)
			if strings.HasPrefix(tag, WEBCOMPONENT_PREFIX) {
				distinctTags[tag] = true
				continue
			}

			if hasAttr && ALLOW_IS_TAG_COMPONENTS {
				for {
					attr, val, moreAttr := tokens.TagAttr()
					if reflect.DeepEqual(attr, IS_TAG) {
						attrValue := string(val)
						if strings.HasPrefix(attrValue, WEBCOMPONENT_PREFIX) {
							distinctTags[attrValue] = true
						}
					}
					if !moreAttr {
						break
					}
				}
			}
		}
	}

	err := tokens.Err()
	if err != nil && err != io.EOF {
		printError("error: could not traverse component: %v\n", err)
	}

	var headContent string
	if isPage {
		headContent = strings.TrimSpace(headBuf.String())
	}

	tags := make([]string, 0, len(distinctTags))
	for key, _ := range distinctTags {
		tags = append(tags, key)
	}

	return tags, headContent
}

func dfsCollectComponents(node *Component, graph ComponentsGraph, visited map[*Component]bool, cmps *[]*Component) {
	visited[node] = true
	*cmps = append(*cmps, node)
	for _, component := range graph[node] {
		if !visited[component] {
			dfsCollectComponents(component, graph, visited, cmps)
		}
	}
}

// func dfsPrintComponents(node *Component, graph ComponentsGraph, visited map[*Component]bool, index int) {
//	visited[node] = true
//	printInfo("%*s%s\n", index*4, "", node.Path)
//	for _, component := range graph[node] {
//		if !visited[component] {
//			dfsPrintComponents(component, graph, visited, index+1)
//		}
//	}
// }

// func printComponentsDeps(content string) {
//	visited := make(map[*Component]bool)
//	deps, _ := collectWebComponentTags(content, false)
//	for _, dep := range deps {
//		component, found := components[dep]
//		if found {
//			if !visited[component] {
//				dfsPrintComponents(component, graph, visited, 0)
//			}
//		} else {
//			// @note: at this point we should already have verified that the
//			// component exists and that the graph has no cyclic references
//			printError("unreachable: component %s is missing\n", dep)
//		}
//	}
// }

func processComponents(root string) map[string]*Component {
	components := make(map[string]*Component)
	err := filepath.WalkDir(root, func(path string, info fs.DirEntry, err error) error {
		if err == nil {
			// @fixme: for now only components ended with .html is allowed
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".html") {
				printInfo("info: parsing %s\n", path)
				component := NewComponent(path)
				components[component.Tag] = component
			}
		}
		return err
	})
	if err != nil {
		printError("error: processing components: %v\n", err)
	}
	return components
}

func copyAssetsFolder(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			printInfo("info: making asset directory %s...\n", dstPath)
			return os.MkdirAll(dstPath, 0755)
		}

		srcStat, err := os.Stat(path)
		if err != nil {
			return err
		}

		dstStat, err := os.Stat(dstPath)
		if err != nil {
			// Destination file does not exist, copy the file
			printInfo("info: copying asset file %s...\n", path)
			return copyFile(path, dstPath, 0755)
		}

		// Destination file exists, check if it's older than the source file
		if dstStat.ModTime().Before(srcStat.ModTime()) {
			printInfo("info: updating asset file %s...\n", path)
			return copyFile(path, dstPath, 0755)
		}

		printInfo("info: asset file %s is up to date, skipping...\n", path)
		// Destination the file is update to date do nothing
		return nil
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
