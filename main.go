package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// -------------------- OpenAPI Data Structures --------------------

type OpenAPI struct {
	OpenAPI    string                `yaml:"openapi"`
	Info       Info                  `yaml:"info"`
	Paths      map[string]PathItem   `yaml:"paths"`
	Servers    []Server              `yaml:"servers,omitempty"`
	Components *Components           `yaml:"components,omitempty"`
	Security   []map[string][]string `yaml:"security,omitempty"`
}

type Components struct {
	SecuritySchemes map[string]SecurityScheme `yaml:"securitySchemes,omitempty"`
}

type SecurityScheme struct {
	Type        string `yaml:"type,omitempty"`
	Scheme      string `yaml:"scheme,omitempty"`
	In          string `yaml:"in,omitempty"`
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
}

type Server struct {
	URL         string                    `yaml:"url"`
	Description string                    `yaml:"description,omitempty"`
	Variables   map[string]ServerVariable `yaml:"variables,omitempty"`
}

type ServerVariable struct {
	Enum        []string `yaml:"enum,omitempty"`
	Default     string   `yaml:"default"`
	Description string   `yaml:"description,omitempty"`
}

type Info struct {
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

type PathItem map[string]Operation

type Operation struct {
	Summary     string              `yaml:"summary,omitempty"`
	Description string              `yaml:"description,omitempty"`
	Tags        []string            `yaml:"tags,omitempty"`
	Parameters  []Parameter         `yaml:"parameters,omitempty"`
	RequestBody *RequestBody        `yaml:"requestBody,omitempty"`
	Responses   map[string]Response `yaml:"responses,omitempty"`
}
type Response struct {
	Description string            `yaml:"description"`
	Content     map[string]Schema `yaml:"content,omitempty"`
}
type Parameter struct {
	Name        string `yaml:"name"`
	In          string `yaml:"in"`
	Description string `yaml:"description,omitempty"`
	Required    bool   `yaml:"required"`
	Example     any    `yaml:"example,omitempty"`
}

type Schema struct {
	Type        string             `yaml:"type,omitempty"`
	Properties  map[string]*Schema `yaml:"properties,omitempty"`
	Items       *Schema            `yaml:"items,omitempty"`
	Required    []string           `yaml:"required,omitempty"`
	Description string             `yaml:"description,omitempty"`
	Format      string             `yaml:"format,omitempty"`
}

type RequestBody struct {
	Required    bool                 `yaml:"required"`
	Content     map[string]MediaType `yaml:"content"`
	Description string               `yaml:"description,omitempty"`
}

type MediaType struct {
	Schema  *Schema `yaml:"schema,omitempty"`
	Example any     `yaml:"example,omitempty"`
}

// -------------------- Helper Functions --------------------

// Write YAML file to disk
func writeYAMLFile(path string, data any) error {
	out, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return err
	}
	fmt.Printf("OpenAPI YAML generated at %s\n", path)
	return nil
}

// Parse a key-value block into a map
func parseKVBlock(block string) map[string]string {
	kv := map[string]string{}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		kv[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return kv
}

// Normalize URL path (remove placeholders)
func normalizePath(url string) string {
	url = strings.Trim(url, `"`)
	url = strings.ReplaceAll(url, "{{host}}", "")
	url = strings.ReplaceAll(url, "{{baseURL}}", "")
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}
	return url
}

// Extract tags from file path relative to root
func tagsFromPath(root, file string) []string {
	dir := filepath.Dir(file)
	rel, _ := filepath.Rel(root, dir)
	if rel == "." {
		return nil
	}
	return strings.Split(rel, string(os.PathSeparator))
}

// Pretty format JSON for description
func formatJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var obj interface{}
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return s
	}
	out, _ := json.MarshalIndent(obj, "", "  ")
	return string(out)
}

// -------------------- JSON Schema Parsing --------------------

// Convert JSON string to Schema
func parseJSONBodyToSchema(body string) *Schema {
	body = strings.TrimSpace(body)
	if body == "" {
		return &Schema{Type: "object"}
	}
	var v interface{}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return &Schema{Type: "string"}
	}
	return parseValueToSchema(v, 0)
}

// Recursive conversion of JSON value to Schema
func parseValueToSchema(v interface{}, depth int) *Schema {
	const maxDepth = 20
	if depth >= maxDepth {
		return &Schema{Type: "object"}
	}
	switch val := v.(type) {
	case string:
		return &Schema{Type: "string"}
	case float64:
		return &Schema{Type: "number"}
	case bool:
		return &Schema{Type: "boolean"}
	case map[string]interface{}:
		props := make(map[string]*Schema)
		for k, child := range val {
			props[k] = parseValueToSchema(child, depth+1)
		}
		return &Schema{Type: "object", Properties: props}
	case []interface{}:
		if len(val) == 0 {
			return &Schema{Type: "array", Items: &Schema{Type: "string"}}
		}
		itemSchema := parseValueToSchema(val[0], depth+1)
		return &Schema{Type: "array", Items: itemSchema}
	default:
		return &Schema{Type: "string"}
	}
}

// -------------------- Request Body Parsing --------------------

// Parse key-value lines into schema properties, required fields, and examples
func parseBodyFields(block string) (map[string]*Schema, []string, map[string]string) {
	props := map[string]*Schema{}
	var required []string
	examples := map[string]string{}
	for _, l := range strings.Split(block, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || !strings.Contains(l, ":") {
			continue
		}
		parts := strings.SplitN(l, ":", 2)
		name := strings.TrimPrefix(strings.TrimSpace(parts[0]), "~")
		if !strings.HasPrefix(parts[0], "~") {
			required = append(required, name)
		}
		value := strings.TrimSpace(parts[1])
		props[name] = &Schema{Description: value}
		examples[name] = value
	}
	return props, required, examples
}

// Parse application/x-www-form-urlencoded body
func requestBodyFormUrlencoded(block string) MediaType {
	props, required, examples := parseBodyFields(block)
	vals := url.Values{}
	for k, v := range examples {
		vals.Set(k, v)
	}
	return MediaType{
		Schema:  &Schema{Type: "object", Properties: props, Required: required},
		Example: vals.Encode(),
	}
}

// Parse multipart/form-data body
func requestBodyMultipart(block string) MediaType {
	props, required, examples := parseBodyFields(block)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for k, v := range examples {
		writer.WriteField(k, v)
	}
	writer.Close()
	return MediaType{
		Schema:  &Schema{Type: "object", Properties: props, Required: required},
		Example: body.String(),
	}
}

// Extract request bodies from all body blocks
func extractRequestBodies(blocks map[string]string) *RequestBody {
	rb := &RequestBody{
		Required: true,
		Content:  map[string]MediaType{},
	}
	for k, b := range blocks {
		if !strings.HasPrefix(k, "body:") {
			continue
		}
		switch k {
		case "body:form-urlencoded":
			rb.Content["application/x-www-form-urlencoded"] = requestBodyFormUrlencoded(b)
		case "body:multipart-form":
			rb.Content["multipart/form-data"] = requestBodyMultipart(b)
		case "body:json":
			rb.Content["application/json"] = MediaType{
				Schema:  parseJSONBodyToSchema(b),
				Example: b,
			}
		case "body:text":
			rb.Content["text/plain"] = MediaType{
				Schema:  &Schema{Type: "string"},
				Example: b,
			}
		case "body:xml":
			rb.Content["application/xml"] = MediaType{
				Example: b,
			}
		default:
			rb.Content["application/octet-stream"] = MediaType{
				Schema: &Schema{Type: "string", Format: "binary"},
			}
		}
	}
	if len(rb.Content) == 0 {
		return nil
	}
	return rb
}

// -------------------- Block Parsing --------------------

// Parse blocks like "meta { ... }"
func parseBlocks(content string) map[string]string {
	block := make(map[string]string)
	reg := regexp.MustCompile(`([\w:-]+) \{\n([\s\S]+?)\n\}\n`)
	result := reg.FindAllStringSubmatch(content, -1)
	for _, r := range result {
		block[r[1]] = r[2]
	}
	return block
}

// -------------------- Parameters --------------------

// Extract parameters from block for query or header
func extractParams(block, in string) []Parameter {
	lines := strings.Split(block, "\n")
	var params []Parameter
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || !strings.Contains(l, ":") {
			continue
		}
		parts := strings.SplitN(l, ":", 2)
		required := !strings.HasPrefix(parts[0], "~")
		name := strings.TrimPrefix(strings.TrimSpace(parts[0]), "~")
		value := strings.TrimSpace(parts[1])
		params = append(params, Parameter{
			Name:     name,
			In:       in,
			Required: required,
			Example:  value,
		})
	}
	return params
}

// -------------------- Environment --------------------

// Parse environment block into Server
func parseEnvironmentBlock(block string) (*Server, bool) {
	server := Server{Variables: make(map[string]ServerVariable)}
	lines := strings.Split(block, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || !strings.Contains(l, ":") {
			continue
		}
		parts := strings.SplitN(l, ":", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "host", "baseURL":
			if server.URL == "" {
				server.URL = value
			}
		default:
			server.Variables[key] = ServerVariable{Default: value}
		}
	}
	if server.URL == "" {
		return nil, false
	}
	return &server, true
}

// -------------------- Auth --------------------

// Parse API key authentication
func authAPIKey(b string, api *OpenAPI) {
	kv := parseKVBlock(b)
	key, in, desc := kv["key"], kv["placement"], kv["value"]
	api.Components.SecuritySchemes["apiKey"] = SecurityScheme{
		Type:        "apiKey",
		Name:        key,
		In:          in,
		Description: desc,
	}
	api.Security = append(api.Security, map[string][]string{"apiKey": {}})
}

// -------------------- Process .bru file --------------------

func processBruFile(root string, file string, api *OpenAPI) {
	data, err := os.ReadFile(file)
	if err != nil {
		log.Printf("read file %s error: %v", file, err)
		return
	}
	content := string(data)
	blocks := parseBlocks(content)

	baseDir := filepath.Dir(file)
	if strings.HasSuffix(baseDir, "environments") {
		if s, ok := parseEnvironmentBlock(blocks["vars"]); ok {
			api.Servers = append(api.Servers, *s)
			return
		}
	}

	var method, URL string
	var op Operation
	tags := tagsFromPath(root, file)

	if meta, ok := blocks["meta"]; ok {
		metaKV := parseKVBlock(meta)
		op.Summary = metaKV["name"]
	}

	if docs, ok := blocks["docs"]; ok {
		op.Description = formatJSON(docs)
	}

	// Auth handling map
	authMap := map[string]func(string){
		"auth:basic": func(b string) {
			api.Components.SecuritySchemes["basicAuth"] = SecurityScheme{Type: "http", Scheme: "basic"}
			api.Security = append(api.Security, map[string][]string{"basicAuth": {}})
		},
		"auth:apikey": func(b string) { authAPIKey(b, api) },
		"auth:bearer": func(b string) {
			api.Components.SecuritySchemes["bearerAuth"] = SecurityScheme{Type: "http", Scheme: "bearer"}
			api.Security = append(api.Security, map[string][]string{"bearerAuth": {}})
		},
		"auth:digest": func(b string) {
			api.Components.SecuritySchemes["digestAuth"] = SecurityScheme{Type: "http", Scheme: "digest"}
			api.Security = append(api.Security, map[string][]string{"digestAuth": {}})
		},
		"auth:wsse": func(b string) {
			api.Components.SecuritySchemes["wsseAuth"] = SecurityScheme{Type: "http", Scheme: "wsse"}
			api.Security = append(api.Security, map[string][]string{"wsseAuth": {}})
		},
	}

	for k, handler := range authMap {
		if b, ok := blocks[k]; ok {
			handler(b)
		}
	}

	for _, m := range []string{"get", "post", "put", "delete"} {
		if block, ok := blocks[m]; ok {
			method = m
			URL = parseKVBlock(block)["url"]
		}
	}

	if URL == "" || method == "" {
		return
	}

	// Extract parameters
	var parameters []Parameter
	if b, ok := blocks["params:query"]; ok {
		parameters = append(parameters, extractParams(b, "query")...)
	}
	if b, ok := blocks["headers"]; ok {
		parameters = append(parameters, extractParams(b, "header")...)
	}
	op.Parameters = parameters

	// Extract request body
	op.RequestBody = extractRequestBodies(blocks)

	// Default response 200
	op.Responses = map[string]Response{
		//"200": {Description: `{"code":0,"message":"success","data":{ }}`},
		"200": {Description: "OK"},
	}
	op.Tags = tags

	path := normalizePath(URL)
	if api.Paths[path] == nil {
		api.Paths[path] = PathItem{}
	}
	api.Paths[path][method] = op
}

// -------------------- Main --------------------

func main() {
	var (
		rootDir    = flag.String("d", "./bru_files", "path to .bru files")
		outPath    = flag.String("o", "./openapi.yaml", "output file path")
		excludeStr = flag.String("exclude", "", "comma separated folders to exclude")
	)
	flag.Parse()

	excludes := map[string]struct{}{}
	for _, e := range strings.Split(*excludeStr, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			excludes[e] = struct{}{}
		}
	}

	api := OpenAPI{
		OpenAPI:    "3.0.0",
		Info:       Info{Title: "API Docs", Version: "1.0.0"},
		Paths:      map[string]PathItem{},
		Components: &Components{SecuritySchemes: map[string]SecurityScheme{}},
	}

	err := filepath.Walk(*rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if _, ok := excludes[info.Name()]; ok {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".bru") {
			processBruFile(*rootDir, path, &api)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := writeYAMLFile(*outPath, api); err != nil {
		log.Fatal(err)
	}
}
