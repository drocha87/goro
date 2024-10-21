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
var components map[string]*Component

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

func (c *Component) PopulateComponents() {
	c.Components = make([]*Component, 0, len(c.WcTags))
	for _, tag := range c.WcTags {
		component, found := components[tag]
		if !found {
			log.Fatalf("error: could not build component %s graph because `%s` is not defined\n", c.Path, tag)
		}
		c.Components = append(c.Components, component)
	}
}

type ComponentsGraph map[*Component][]*Component

var graph ComponentsGraph

func buildComponentsGraph() {
	graph = make(ComponentsGraph, 0)
	for _, value := range components {
		graph[value] = value.Components
	}
}

func dfsForCyclicComponents(node *Component, graph ComponentsGraph, visited map[*Component]bool, stack map[*Component]bool, trace *[]*Component) bool {
	visited[node] = true
	stack[node] = true
	*trace = append(*trace, node)

	// Traverse all dependencies of the current component
	for _, component := range graph[node] {
		if !visited[component] {
			if dfsForCyclicComponents(component, graph, visited, stack, trace) {
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
func failOnCyclicComponents(graph ComponentsGraph) {
	visited := make(map[*Component]bool)
	stack := make(map[*Component]bool)
	trace := make([]*Component, 0)

	// Run DFS for every node that hasn't been visited
	for component := range graph {
		if !visited[component] {
			if dfsForCyclicComponents(component, graph, visited, stack, &trace) {
				log.Println("\nerror: cyclic reference found")
				for i, c := range trace {
					log.Printf("%*s%s\n", (i+1)*2, "", c.Path)
				}
				os.Exit(1)
			}
		}
	}
}

func main() {
	log.SetFlags(0)

	usedIDS = make(map[string]bool)
	components = make(map[string]*Component, 0)

	// First pass: Traverse the `COMPONENTS_BASEPATH` to generate a `Component`.
	// In this phase, only the `Tag`, `Name`, `Path`, `Content`, and `WcTags` fields are loaded
	// and appended to `components`. The `Components` slice is not fully populated after this call.
	parseFiles(COMPONENTS_BASEPATH, false)

	// This is necessary to build the components graph used to check for
	// cyclic or recursive components
	for _, component := range components {
		component.PopulateComponents()
	}

	log.Println("\nchecking for cyclic or recursive component...")

	buildComponentsGraph()

	// traverse the graph looking for cycles using Depth-First-Search
	// algorithm, if we found a cycle we exit with a failure message
	failOnCyclicComponents(graph)

	log.Println("done...\n")

	createBuildFolder()

	// Note: This must be executed after parseFiles(COMPONENTS_BASEPATH,
	// false) to ensure the component dependency is defined. Running it
	// before will cause failure.
	parseFiles(PAGES_BASEPATH, true)

	// copy asset files
	err := copyAssetsFolder(ASSETS_BASEPATH, fmt.Sprintf("%s/assets", DIST_BASEPATH))
	if err != nil {
		log.Printf("error: could not finish copying %s folder: %v\n", ASSETS_BASEPATH, err)
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
			log.Fatalf("error: could not generate random indentifier: %v\n", err)
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
		log.Fatal(err)
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
		log.Fatalf("error: could not read %s content: %v\n", path, err)
	}
	tmpl, err = tmpl.Parse(string(content[:]))
	if err != nil {
		log.Fatalf("error: could not parse template %s: %v\n", path, err)
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
					log.Fatalf("error: <head> tag is not allowed on component!")
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
		log.Fatalf("error: could not traverse component: %v\n", err)
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

func loadComponent(path string) {
	comp := &Component{Path: path}

	comp.Name = strings.Split(filepath.Base(comp.Path), ".")[0]
	comp.Tag = strings.Replace(comp.Name, "_", "-", -1)

	if !strings.HasPrefix(comp.Tag, WEBCOMPONENT_PREFIX) {
		// @todo: improve error message
		log.Fatalf("error: web component %s must be prefixed with %s", path, WEBCOMPONENT_PREFIX)
	}

	tmpl := loadTemplateFile(comp.Path)

	var buf strings.Builder
	uniqueID := GenerateUniqueID()
	if err := tmpl.Execute(&buf, map[string]any{"Tag": comp.Tag, "UniqueID": uniqueID}); err != nil {
		log.Fatalf("error: could build component %s: %v", comp.Path, err)
	}
	contentStr := buf.String()

	comp.Content = template.HTML(contentStr)
	comp.WcTags, _ = collectWebComponentTags(contentStr, false)

	components[comp.Tag] = comp
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

func collectPageComponents(path string, content string) ([]*Component, string) {
	visited := make(map[*Component]bool)
	cmps := make([]*Component, 0)
	deps, headContent := collectWebComponentTags(content, true)

	for _, dep := range deps {
		component, found := components[dep]
		if found {
			if !visited[component] {
				dfsCollectComponents(component, graph, visited, &cmps)
			}
		} else {
			log.Fatalf("error: page %s requires component %s which is missing\n", path, dep)
		}
	}
	return cmps, headContent
}

func dfsPrintComponents(node *Component, graph ComponentsGraph, visited map[*Component]bool, index int) {
	visited[node] = true
	log.Printf("%*s%s\n", index*4, "", node.Path)
	for _, component := range graph[node] {
		if !visited[component] {
			dfsPrintComponents(component, graph, visited, index+1)
		}
	}
}

func printComponentsDeps(content string) {
	visited := make(map[*Component]bool)
	deps, _ := collectWebComponentTags(content, false)
	for _, dep := range deps {
		component, found := components[dep]
		if found {
			if !visited[component] {
				dfsPrintComponents(component, graph, visited, 0)
			}
		} else {
			// @note: at this point we should already have verified that the
			// component exists and that the graph has no cyclic references
			log.Fatalf("unreachable: component %s is missing\n", dep)
		}
	}
}

type AppData struct {
	Components []*Component
	Title      string
	Content    template.HTML
	Head       template.HTML
}

func compilePage(path string) {
	dest := filepath.Clean(fmt.Sprintf("%s/%s", DIST_BASEPATH, strings.TrimPrefix(path, PAGES_BASEPATH)))
	tmpl := loadTemplateFile(path)

	var buf strings.Builder
	uniqueID := GenerateUniqueID()
	if err := tmpl.Execute(&buf, map[string]any{"UniqueID": uniqueID}); err != nil {
		log.Fatalf("error: could not compile page %s: %v", path, err)
	}
	content := buf.String()

	deps, headContent := collectPageComponents(path, content)
	// printComponentsDeps(content)

	data := AppData{
		Title:      "ECB Dashboard",
		Components: deps,
		Content:    template.HTML(content),
		Head:       template.HTML(headContent),
	}

	appTmpl := loadTemplateFile("./app.html")
	buf.Reset()
	if err := appTmpl.Execute(&buf, &data); err != nil {
		log.Fatalf("error: could compile %s with app.html: %v", path, err)
	}

	if _, err := os.Stat(dest); os.IsNotExist(err) {
		dir := filepath.Dir(dest)
		log.Printf("info: creating dir %s\n", dir)
		os.MkdirAll(dir, 0755)
	}

	err := os.WriteFile(dest, []byte(buf.String()), 0755)
	if err != nil {
		log.Fatalf("error: could not create file %s: %v\n", dest, err)
	}

	log.Printf("generated %s -> %s\n", path, dest)
}

func parseFiles(root string, isPages bool) {
	err := filepath.WalkDir(root, func(path string, info fs.DirEntry, err error) error {
		if err == nil {
			// @fixme: for now only components ended with .html is allowed
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".html") {
				if isPages {
					compilePage(path)
				} else {
					log.Printf("info: parsing %s\n", path)
					loadComponent(path)
				}
			}
		}
		return err
	})
	if err != nil {
		log.Fatalf("error: loading components: %v\n", err)
	}
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
			log.Printf("info: making asset directory %s...\n", dstPath)
			return os.MkdirAll(dstPath, 0755)
		}

		srcStat, err := os.Stat(path)
		if err != nil {
			return err
		}

		dstStat, err := os.Stat(dstPath)
		if err != nil {
			// Destination file does not exist, copy the file
			log.Printf("info: copying asset file %s...\n", path)
			return copyFile(path, dstPath, 0755)
		}

		// Destination file exists, check if it's older than the source file
		if dstStat.ModTime().Before(srcStat.ModTime()) {
			log.Printf("info: updating asset file %s...\n", path)
			return copyFile(path, dstPath, 0755)
		}

		log.Printf("info: asset file %s is up to date, skipping...\n", path)
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
