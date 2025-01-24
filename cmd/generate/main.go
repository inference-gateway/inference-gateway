package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"html/template"
	"os/exec"

	"os"
	"reflect"
	"strings"

	config "github.com/inference-gateway/inference-gateway/config"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

var (
	output string
	_type  string
)

// OpenAPI schema structures
type OpenAPISchema struct {
	Components struct {
		Schemas struct {
			Providers struct {
				XProviderConfigs map[string]ProviderConfig `yaml:"x-provider-configs"`
			} `yaml:"Providers"`
		}
	}
}

// ExtraHeader can be either string or []string
type ExtraHeader struct {
	Values []string
}

// UnmarshalYAML implements custom unmarshaling for ExtraHeader
func (h *ExtraHeader) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		h.Values = []string{value.Value}
	case yaml.SequenceNode:
		var values []string
		if err := value.Decode(&values); err != nil {
			return err
		}
		h.Values = values
	default:
		return fmt.Errorf("unexpected header value type")
	}
	return nil
}

type ProviderEndpoints struct {
	List     string `yaml:"list"`
	Generate string `yaml:"generate"`
}

// Structures for OpenAPI schema parsing
type SchemaProperty struct {
	Type       string                 `yaml:"type"`
	Properties map[string]SchemaField `yaml:"properties"`
	Items      *SchemaField           `yaml:"items"`
	Ref        string                 `yaml:"$ref"`
}

type SchemaField struct {
	Type       string                 `yaml:"type"`
	Properties map[string]SchemaField `yaml:"properties"`
	Items      *SchemaField           `yaml:"items"`
	Ref        string                 `yaml:"$ref"`
}

type EndpointSchema struct {
	Endpoint string `yaml:"endpoint"`
	Method   string `yaml:"method"`
	Schema   struct {
		Request  SchemaProperty `yaml:"request"`
		Response SchemaProperty `yaml:"response"`
	} `yaml:"schema"`
}

type ProviderConfig struct {
	URL          string                    `yaml:"url"`
	AuthType     string                    `yaml:"auth_type"`
	ExtraHeaders map[string]ExtraHeader    `yaml:"extra_headers,omitempty"`
	Endpoints    map[string]EndpointSchema `yaml:"endpoints"`
}

func init() {
	flag.StringVar(&output, "output", "", "Path to the output file")
	flag.StringVar(&_type, "type", "", "The type of the file to generate (Env, ConfigMap, Secret, or MD)")
}

func main() {
	flag.Parse()

	if output == "" || _type == "" {
		fmt.Println("Both -output and -type must be specified")
		os.Exit(1)
	}

	switch _type {
	case "Env":
		comments := parseStructComments("config.go", "Config")
		generateEnvExample(output, comments)
	case "ConfigMap":
		comments := parseStructComments("config.go", "Config")
		generateConfigMap(output, comments)
	case "Secret":
		comments := parseStructComments("config.go", "Config")
		generateSecret(output, comments)
	case "MD":
		comments := parseStructComments("config.go", "Config")
		generateMD(output, comments)
	case "Provider":
		if err := generateProviders(output, "openapi.yaml"); err != nil {
			fmt.Printf("Error generating providers: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Println("Invalid type specified")
		os.Exit(1)
	}
}

func parseStructComments(filename, structName string) map[string]string {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		fmt.Printf("Error parsing file: %v\n", err)
		os.Exit(1)
	}

	comments := make(map[string]string)
	ast.Inspect(node, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != structName {
			return true
		}

		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}

		for _, field := range st.Fields.List {
			if field.Doc != nil {
				for _, comment := range field.Doc.List {
					comments[field.Names[0].Name] = strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
				}
			}
		}
		return false
	})

	return comments
}

func generateEnvExample(filePath string, comments map[string]string) {
	var cfg config.Config
	v := reflect.ValueOf(cfg)
	t := v.Type()

	var sb strings.Builder
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		envTag := field.Tag.Get("env")
		if envTag == "" {
			continue
		}
		envParts := strings.Split(envTag, ",")
		envName := envParts[0]
		defaultValue := ""
		for _, part := range envParts {
			part = strings.Trim(part, " ")
			if strings.HasPrefix(part, "default=") {
				defaultValue = strings.TrimPrefix(part, "default=")
				break
			}
		}
		if comment, ok := comments[field.Name]; ok {
			sb.WriteString(fmt.Sprintf("# %s\n", comment))
		}
		sb.WriteString(fmt.Sprintf("%s=%s\n", envName, defaultValue))
	}

	err := os.WriteFile(filePath, []byte(sb.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing %s: %v\n", filePath, err)
	}
}

func generateConfigMap(filePath string, comments map[string]string) {
	var cfg config.Config
	v := reflect.ValueOf(cfg)
	t := v.Type()

	var sb strings.Builder
	sb.WriteString("---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: inference-gateway\n  namespace: inference-gateway\n  labels:\n    app: inference-gateway\ndata:\n")

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		envTag := field.Tag.Get("env")
		typeTag := field.Tag.Get("type")
		if typeTag == "secret" {
			continue
		}

		if envTag == "" {
			continue
		}
		envParts := strings.Split(envTag, ",")
		envName := envParts[0]

		defaultValue := ""
		for _, part := range envParts {
			part = strings.Trim(part, " ")
			if strings.HasPrefix(part, "default=") {
				if envName == "OLLAMA_API_URL" {
					defaultValue = "http://ollama.ollama:8080"
					break
				}

				if envName == "OIDC_ISSUER_URL" {
					defaultValue = "http://keycloak.keycloak:8080/realms/inference-gateway-realm"
					break
				}

				defaultValue = strings.TrimPrefix(part, "default=")
				break
			}
		}
		if comment, ok := comments[field.Name]; ok {
			sb.WriteString(fmt.Sprintf("  # %s\n", comment))
		}
		sb.WriteString(fmt.Sprintf("  %s: \"%s\"\n", envName, defaultValue))
	}

	err := os.WriteFile(filePath, []byte(sb.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing %s: %v\n", filePath, err)
	}
}

func generateSecret(filePath string, comments map[string]string) {
	var cfg config.Config
	v := reflect.ValueOf(cfg)
	t := v.Type()

	var sb strings.Builder
	sb.WriteString("---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: inference-gateway\n  namespace: inference-gateway\n  labels:\n    app: inference-gateway\ntype: Opaque\nstringData:\n")

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		envTag := field.Tag.Get("env")
		typeTag := field.Tag.Get("type")
		if typeTag != "secret" {
			continue
		}

		if envTag == "" {
			continue
		}
		envParts := strings.Split(envTag, ",")
		envName := envParts[0]

		if comment, ok := comments[field.Name]; ok {
			sb.WriteString(fmt.Sprintf("  # %s\n", comment))
		}
		sb.WriteString(fmt.Sprintf("  %s: \"\"\n", envName))
	}

	err := os.WriteFile(filePath, []byte(sb.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing %s: %v\n", filePath, err)
	}
}

func generateMD(filePath string, comments map[string]string) {
	var cfg config.Config
	v := reflect.ValueOf(cfg)
	t := v.Type()

	var sb strings.Builder
	sb.WriteString("# Inference Gateway Configuration\n")

	currentGroup := ""
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		envTag := field.Tag.Get("env")
		if envTag == "" {
			continue
		}
		envParts := strings.Split(envTag, ",")
		envName := envParts[0]
		description := field.Tag.Get("description")
		defaultValue := ""
		for _, part := range envParts {
			part = strings.Trim(part, " ")
			if strings.HasPrefix(part, "default=") {
				defaultValue = strings.TrimPrefix(part, "default=")
				break
			}
		}

		group := comments[field.Name]
		if group != currentGroup {
			if group != "" {
				sb.WriteString("\n")
				sb.WriteString(fmt.Sprintf("## %s\n\n", group))
				sb.WriteString("| Key | Default Value | Description |\n")
				sb.WriteString("| --- | ------------- | ----------- |\n")
				currentGroup = group
			}
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", envName, defaultValue, description))
	}

	err := os.WriteFile(filePath, []byte(sb.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing %s: %v\n", filePath, err)
	}
}

func generateProviders(output string, openapiPath string) error {
	// Read OpenAPI spec
	data, err := os.ReadFile(openapiPath)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	var schema OpenAPISchema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	providers := schema.Components.Schemas.Providers.XProviderConfigs

	// Generate provider files
	for name, config := range providers {
		if err := generateProviderFile(output, name, config); err != nil {
			return fmt.Errorf("failed to generate provider %s: %w", name, err)
		}
	}

	return nil
}

func generateProviderFile(destination, name string, config ProviderConfig) error {
	caser := cases.Title(language.English)

	funcMap := template.FuncMap{
		"title":        caser.String,
		"generateType": generateType,
	}

	tmpl := template.Must(template.New("provider").
		Funcs(funcMap).
		Parse(`package providers

{{- if .Config.ExtraHeaders }}
// Extra headers for {{title .Name}} provider
var {{title .Name}}ExtraHeaders = map[string][]string{
    {{- range $key, $header := .Config.ExtraHeaders}}
    "{{$key}}": {"{{index $header.Values 0}}"},
    {{- end}}
}
{{end}}

{{- with .Config.Endpoints.list.Schema.Response }}
type GetModelsResponse{{title $.Name}} struct {
    {{- if eq .Type "object" }}
    {{- range $key, $prop := .Properties }}
    {{title $key}} {{generateType $prop}} ` + "`json:\"{{$key}}\"`" + `
    {{- end }}
    {{- end }}
}
{{end}}

{{- with .Config.Endpoints.generate.Schema }}
{{- if .Request.Properties }}
type GenerateRequest{{title $.Name}} struct {
    {{- range $key, $prop := .Request.Properties }}
    {{title $key}} {{generateType $prop}} ` + "`json:\"{{$key}}\"`" + `
    {{- end }}
}
{{end}}

{{- if .Response.Properties }}
type GenerateResponse{{title $.Name}} struct {
    {{- range $key, $prop := .Response.Properties }}
    {{title $key}} {{generateType $prop}} ` + "`json:\"{{$key}}\"`" + `
    {{- end }}
}
{{end}}
{{end}}`))

	data := struct {
		Name   string
		Config ProviderConfig
	}{
		Name:   name,
		Config: config,
	}

	fileName := fmt.Sprintf("%s/%s.go", destination, strings.ToLower(name))
	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	// Run go fmt on the generated file
	cmd := exec.Command("go", "fmt", fileName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to format %s: %w", fileName, err)
	}

	return nil
}

func generateType(field SchemaField) string {
	switch field.Type {
	case "string":
		return "string"
	case "integer":
		return "int"
	case "boolean":
		return "bool"
	case "array":
		if field.Items != nil {
			return "[]" + generateType(*field.Items)
		}
		return "[]interface{}"
	case "object":
		if len(field.Properties) > 0 {
			return "struct{}"
		}
		return "map[string]interface{}"
	default:
		if field.Ref != "" {
			parts := strings.Split(field.Ref, "/")
			return parts[len(parts)-1]
		}
		return "interface{}"
	}
}
