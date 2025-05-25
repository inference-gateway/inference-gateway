package codegen

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/inference-gateway/inference-gateway/internal/openapi"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// GenerateConfig generates a configuration file from an OpenAPI spec
func GenerateConfig(destination string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	caser := cases.Title(language.English)

	funcMap := template.FuncMap{
		"title":      caser.String,
		"upper":      strings.ToUpper,
		"trimPrefix": strings.TrimPrefix,
		"pascalCase": func(s string) string {
			parts := strings.Split(s, "_")
			for i, part := range parts {
				parts[i] = cases.Title(language.English).String(strings.ToLower(part))
			}
			return strings.Join(parts, "")
		},
	}

	tmpl := template.Must(template.New("config").Funcs(funcMap).Parse(`// Code generated from OpenAPI schema. DO NOT EDIT.
package config
	
import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/sethvargo/go-envconfig"
)

// Config holds the configuration for the Inference Gateway
type Config struct {
	{{- range $section := .Sections }}
	{{- range $name, $section := $section }}
	{{- if eq $name "general" }}
	// {{ $section.Title }}
	{{- range $field := $section.Settings }}
	{{ pascalCase $field.Env }} {{ $field.Type }} ` + "`env:\"{{ $field.Env }}{{if $field.Default}}, default={{$field.Default}}{{end}}\" description:\"{{$field.Description}}\"`" + `
	{{- end }}
	{{- else if eq $name "mcp" }}
	// MCP settings
	MCP *MCPConfig ` + "`env:\", prefix=MCP_\" description:\"MCP configuration\"`" + `
	{{- else if eq $name "oidc" }}
	// OIDC settings
	OIDC *OIDC ` + "`env:\", prefix=OIDC_\" description:\"OIDC configuration\"`" + `
	{{- else if eq $name "server" }}
	// Server settings
	Server *ServerConfig ` + "`env:\", prefix=SERVER_\" description:\"Server configuration\"`" + `
	{{- else if eq $name "client" }}
	// Client settings
	Client *ClientConfig ` + "`env:\", prefix=CLIENT_\" description:\"Client configuration\"`" + `
	{{- end }}
	{{- end }}
	{{- end }}

	// Providers map
	Providers map[providers.Provider]*providers.Config
}

{{- range $section := .Sections }}
{{- range $name, $section := $section }}
{{- if eq $name "mcp" }}

// MCP configuration
type MCPConfig struct {
	{{- range $field := $section.Settings }}
	{{ pascalCase (trimPrefix $field.Env "MCP_") }} {{ $field.Type }} ` + "`env:\"{{ trimPrefix $field.Env \"MCP_\" }}{{if $field.Default}}, default={{$field.Default}}{{end}}\" description:\"{{$field.Description}}\"`" + `
	{{- end }}
}
{{- else if eq $name "oidc" }}

// OIDC configuration
type OIDC struct {
	{{- range $field := $section.Settings }}
	{{ pascalCase (trimPrefix $field.Env "OIDC_") }} string ` + "`env:\"{{ trimPrefix $field.Env \"OIDC_\" }}{{if $field.Default}}, default={{$field.Default}}{{end}}\"{{if $field.Secret}} type:\"secret\"{{end}} description:\"{{$field.Description}}\"`" + `
	{{- end }}
}
{{- else if eq $name "server" }}

// Server configuration
type ServerConfig struct {
	{{- range $field := $section.Settings }}
	{{ pascalCase (trimPrefix $field.Env "SERVER_") }} {{ $field.Type }} ` + "`env:\"{{ trimPrefix $field.Env \"SERVER_\" }}{{if $field.Default}}, default={{$field.Default}}{{end}}\" description:\"{{$field.Description}}\"`" + `
	{{- end }}
}
{{- else if eq $name "client" }}

// Client configuration
type ClientConfig struct {
	{{- range $field := $section.Settings }}
	{{ pascalCase (trimPrefix $field.Env "CLIENT_") }} {{ $field.Type }} ` + "`env:\"{{ trimPrefix $field.Env \"CLIENT_\" }}{{if $field.Default}}, default={{$field.Default}}{{end}}\" description:\"{{$field.Description}}\"`" + `
	{{- end }}
}
{{- end }}
{{- end }}
{{- end }}

// Load configuration
func (cfg *Config) Load(lookuper envconfig.Lookuper) (Config, error) {
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:   cfg,
		Lookuper: lookuper,
	}); err != nil {
		return Config{}, err
	}

	// Initialize Providers map if nil
	if cfg.Providers == nil {
		cfg.Providers = make(map[providers.Provider]*providers.Config)
	}

	// Set defaults for each provider
	for id, defaults := range providers.Registry {
		if _, exists := cfg.Providers[id]; !exists {
			providerCfg := defaults
			url, ok := lookuper.Lookup(strings.ToUpper(string(id)) + "_API_URL")
			if ok {
				providerCfg.URL = url
			}

			token, ok := lookuper.Lookup(strings.ToUpper(string(id)) + "_API_KEY")
			if !ok {
				println("Warn: provider " + id + " is not configured")
			}
			providerCfg.Token = token
			cfg.Providers[id] = providerCfg
		}
	}

	return *cfg, nil
}

// The string representation of Config
func (cfg *Config) String() string {
    return fmt.Sprintf(
        "Config{ApplicationName:%s, Version:%s Environment:%s, EnableTelemetry:%t, EnableAuth:%t, "+
            "MCP:%+v, OIDC:%+v, Server:%+v, Client:%+v, Providers:%+v}",
        APPLICATION_NAME,
        VERSION,
        cfg.Environment,
        cfg.EnableTelemetry,
        cfg.EnableAuth,
        cfg.MCP,
        cfg.OIDC,
        cfg.Server,
        cfg.Client,
        cfg.Providers,
    )
}

`))

	data := struct {
		Sections  []map[string]openapi.Section
		Providers map[string]openapi.ProviderConfig
	}{
		Sections:  schema.Components.Schemas.Config.XConfig.Sections,
		Providers: schema.Components.Schemas.Provider.XProviderConfigs,
	}

	f, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	cmd := exec.Command("go", "fmt", destination)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to format %s: %w", destination, err)
	}

	return nil
}

// GenerateCommonTypes generates common types from OpenAPI spec
func GenerateCommonTypes(destination string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	caser := cases.Title(language.English)

	funcMap := template.FuncMap{
		"title": caser.String,
		"pascalCase": func(s string) string {
			if strings.ToLower(s) == "id" {
				return "ID"
			}

			parts := strings.Split(s, "_")
			for i, part := range parts {
				parts[i] = cases.Title(language.English).String(strings.ToLower(part))
			}
			return strings.Join(parts, "")
		},
		"generateType": generateType,
		"generateTag":  generateTag,
		"hasPrefix":    strings.HasPrefix,
		"hasKey": func(m map[string]openapi.SchemaProperty, key string) bool {
			_, ok := m[key]
			return ok
		},
		"isRequired": func(field string, requiredFields []string) bool {
			for _, rf := range requiredFields {
				if rf == field {
					return true
				}
			}
			return false
		},
	}

	tmpl := template.Must(template.New("common").
		Funcs(funcMap).
		Parse(`// Code generated from OpenAPI schema. DO NOT EDIT.
package providers

// The authentication type of the specific provider
const (
    AuthTypeBearer  = "bearer"
    AuthTypeXheader = "xheader"
    AuthTypeQuery   = "query"
    AuthTypeNone    = "none"
)

// The default base URLs of each provider
const (
    {{- range $name, $config := .Providers }}
    {{title $name}}DefaultBaseURL = "{{$config.URL}}"
    {{- end }}
)

// The default endpoints of each provider
const (
    {{- range $name, $config := .Providers }}
    {{title $name}}ModelsEndpoint = "{{(index $config.Endpoints "models").Endpoint}}"
    {{title $name}}ChatEndpoint   = "{{(index $config.Endpoints "chat").Endpoint}}"
    {{- end }}
)

type Provider string

// The ID's of each provider
const (
    {{- range $name, $config := .Providers }}
    {{title $name}}ID Provider = "{{$config.ID}}"
    {{- end }}
)

// Display names for providers
const (
    {{- range $name, $config := .Providers }}  
    {{title $name}}DisplayName = "{{title $name}}"
    {{- end }}
)

// MessageRole represents the role of a message sender
type MessageRole string

// Message role enum values
const (
    MessageRoleSystem    MessageRole = "system"
    MessageRoleUser      MessageRole = "user"
    MessageRoleAssistant MessageRole = "assistant"
    MessageRoleTool      MessageRole = "tool"
)

// ChatCompletionToolType represents a value type of a Tool in the API
type ChatCompletionToolType string

// ChatCompletionTool represents tool types in the API, currently only function supported
const (
    ChatCompletionToolTypeFunction ChatCompletionToolType = "function"
)

// FinishReason represents the reason for finishing a chat completion
type FinishReason string

// Chat completion finish reasons
const (
    FinishReasonStop          FinishReason = "stop"
    FinishReasonLength        FinishReason = "length"
    FinishReasonToolCalls     FinishReason = "tool_calls"
    FinishReasonContentFilter FinishReason = "content_filter"
)

// ListModelsTransformer interface for transforming provider-specific responses
type ListModelsTransformer interface {
    Transform() ListModelsResponse
}

{{- range $name, $schema := .Schemas }}
{{- if eq (len $schema.Enum) 0 }}
{{- if ne $name "Config" }}
{{- if ne $name "Providers" }}

// {{$name}} represents a {{$name}} in the API
{{- if $schema.AdditionalProperties }}
type {{$name}} map[string]interface{}
{{- else }}
type {{$name}} struct {
    {{- range $field, $prop := $schema.Properties }}
    {{- if not (hasPrefix $field "x-") }}
    {{pascalCase $field}} {{if not (isRequired $field $schema.Required)}}*{{end}}{{generateType $prop}} {{generateTag $field $prop $schema.Required}}
    {{- end }}
    {{- end }}
}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

// Transform converts provider-specific response to common format
func (p *CreateChatCompletionResponse) Transform() CreateChatCompletionResponse {
    return *p
}

`))

	data := struct {
		Providers map[string]openapi.ProviderConfig
		Schemas   map[string]openapi.SchemaProperty
	}{
		Providers: schema.Components.Schemas.Provider.XProviderConfigs,
		Schemas:   openapi.GetSchemas(schema),
	}

	f, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(schema.Components.Schemas.Provider.XProviderConfigs) == 0 {
		return fmt.Errorf("no provider configurations found in OpenAPI spec")
	}

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	cmd := exec.Command("go", "fmt", destination)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: Failed to format %s: %v\n", destination, err)
	}

	return nil
}

func GenerateProvidersClientConfig(destination, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	var clientSection openapi.Section
	for _, sectionMap := range schema.Components.Schemas.Config.XConfig.Sections {
		for name, section := range sectionMap {
			if name == "client" {
				clientSection = section
				break
			}
		}
	}

	if clientSection.Title == "" {
		return fmt.Errorf("client configuration not found in OpenAPI spec")
	}

	funcMap := template.FuncMap{
		"pascalCase": func(s string) string {
			parts := strings.Split(s, "_")
			for i, part := range parts {
				parts[i] = cases.Title(language.English).String(strings.ToLower(part))
			}
			return strings.Join(parts, "")
		},
	}

	const clientTemplate = `// Code generated from OpenAPI schema. DO NOT EDIT.
package providers

import (
    "context"
    "crypto/tls"
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/sethvargo/go-envconfig"
)

//go:generate mockgen -source=client.go -destination=../tests/mocks/client.go -package=mocks
type Client interface {
    Do(req *http.Request) (*http.Response, error)
    Get(url string) (*http.Response, error)
    Post(url string, bodyType string, body string) (*http.Response, error)
}

type ClientImpl struct {
    scheme   string
    hostname string
    port     string
    client   *http.Client
}

type ClientConfig struct {
    {{- range $setting := .ClientSettings }}
    {{ pascalCase $setting.Env }} {{ $setting.Type }} ` + "`env:\"{{ $setting.Env }}, default={{ $setting.Default }}\" description:\"{{ $setting.Description }}\"`" + `
    {{- end }}
}

func NewClientConfig() (*ClientConfig, error) {
    var cfg ClientConfig
    if err := envconfig.Process(context.Background(), &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

func NewHTTPClient(cfg *ClientConfig, scheme, hostname, port string) Client {
    var tlsMinVersion uint16 = tls.VersionTLS12
    if cfg.ClientTlsMinVersion == "TLS13" {
        tlsMinVersion = tls.VersionTLS13
    }

    httpClient := &http.Client{
        Timeout: cfg.ClientTimeout,
        Transport: &http.Transport{
            MaxIdleConns:        cfg.ClientMaxIdleConns,
            MaxIdleConnsPerHost: cfg.ClientMaxIdleConnsPerHost,
            IdleConnTimeout:     cfg.ClientIdleConnTimeout,
            TLSClientConfig: &tls.Config{
                MinVersion: tlsMinVersion,
            },
			ForceAttemptHTTP2: true,
        },
    }

    return &ClientImpl{
        scheme:   scheme,
        hostname: hostname,
        port:     port,
        client:   httpClient,
    }
}

func (c *ClientImpl) Do(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme == "" {
		req.URL.Scheme = c.scheme
	}
	if req.URL.Host == "" {
		req.URL.Host = c.hostname + ":" + c.port
	}

    return c.client.Do(req)
}

func (c *ClientImpl) Get(url string) (*http.Response, error) {
    fullURL := c.scheme + "://" + c.hostname + ":" + c.port + "/" + strings.TrimPrefix(url, "/")
    return c.client.Get(fullURL)
}

func (c *ClientImpl) Post(url string, bodyType string, body string) (*http.Response, error) {
    fullURL := c.scheme + "://" + c.hostname + ":" + c.port + "/" + strings.TrimPrefix(url, "/")
    req, err := http.NewRequest("POST", fullURL, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", bodyType)
    req.Body = io.NopCloser(strings.NewReader(body))
    return c.client.Do(req)
}`

	tmpl, err := template.New("client").Funcs(funcMap).Parse(clientTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	data := struct {
		ClientSettings []openapi.Setting
	}{
		ClientSettings: clientSection.Settings,
	}

	f, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	cmd := exec.Command("go", "fmt", destination)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to format %s: %w", destination, err)
	}

	return nil
}

// GenerateMCPTypes generates Go types from MCP JSON/YAML schema
func GenerateMCPTypes(destination string, schemaPath string) error {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read MCP schema: %w", err)
	}

	var schema map[string]interface{}

	switch {
	case strings.HasSuffix(schemaPath, ".json"):
		if err := json.Unmarshal(data, &schema); err != nil {
			return fmt.Errorf("failed to parse JSON schema: %w", err)
		}
	case strings.HasSuffix(schemaPath, ".yaml"), strings.HasSuffix(schemaPath, ".yml"):
		if err := yaml.Unmarshal(data, &schema); err != nil {
			return fmt.Errorf("failed to parse YAML schema: %w", err)
		}
	default:
		return fmt.Errorf("unsupported schema format: must be .json, .yaml, or .yml")
	}

	definitions, ok := schema["definitions"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("schema does not contain definitions")
	}

	outputFile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	header := `// Code generated from MCP schema. DO NOT EDIT.
package mcp

`
	if _, err := outputFile.WriteString(header); err != nil {
		return fmt.Errorf("failed to write file header: %w", err)
	}

	processedTypes := map[string]bool{}

	acronyms := map[string]bool{
		"id":      true,
		"uri":     true,
		"url":     true,
		"api":     true,
		"html":    true,
		"http":    true,
		"https":   true,
		"json":    true,
		"jsonrpc": true,
		"rpc":     true,
		"mime":    true,
	}

	typeNames := make([]string, 0, len(definitions))
	for typeName := range definitions {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)

	for _, typeName := range typeNames {
		definition := definitions[typeName]

		defMap, ok := definition.(map[string]interface{})
		if !ok {
			continue
		}

		isEnum := false
		var enumValues []interface{}
		if enum, ok := defMap["enum"].([]interface{}); ok && len(enum) > 0 {
			isEnum = true
			enumValues = enum
		}

		if !isEnum {
			continue
		}

		description := ""
		if desc, ok := defMap["description"].(string); ok {
			description = desc
		}

		if description != "" {
			formattedDescription := formatDescription(description)
			if _, err := outputFile.WriteString(formattedDescription + "\n"); err != nil {
				return err
			}
		}

		typeStr := "string"
		if t, ok := defMap["type"].(string); ok {
			typeStr = t
		}

		typeDecl := fmt.Sprintf("type %s %s\n\n", typeName, typeStr)
		if _, err := outputFile.WriteString(typeDecl); err != nil {
			return err
		}

		constDecl := fmt.Sprintf("// %s enum values\nconst (\n", typeName)
		if _, err := outputFile.WriteString(constDecl); err != nil {
			return err
		}

		enumStrings := make([]string, 0, len(enumValues))
		for _, val := range enumValues {
			if strVal, ok := val.(string); ok {
				enumStrings = append(enumStrings, strVal)
			}
		}
		sort.Strings(enumStrings)

		for _, val := range enumStrings {
			enumVal := fmt.Sprintf("\t%s%s %s = \"%s\"\n", typeName, convertToGoFieldName(val, acronyms), typeName, val)
			if _, err := outputFile.WriteString(enumVal); err != nil {
				return err
			}
		}

		if _, err := outputFile.WriteString(")\n\n"); err != nil {
			return err
		}

		processedTypes[typeName] = true
	}

	for _, typeName := range typeNames {
		definition := definitions[typeName]

		defMap, ok := definition.(map[string]interface{})
		if !ok {
			continue
		}

		if processedTypes[typeName] {
			continue
		}

		description := ""
		if desc, ok := defMap["description"].(string); ok {
			description = desc
		}

		if description != "" {
			formattedDescription := formatDescription(description)
			if _, err := outputFile.WriteString(formattedDescription + "\n"); err != nil {
				return err
			}
		}

		structDef := fmt.Sprintf("type %s struct {\n", typeName)
		if _, err := outputFile.WriteString(structDef); err != nil {
			return err
		}

		properties, ok := defMap["properties"].(map[string]interface{})
		if ok {
			propNames := make([]string, 0, len(properties))
			for propName := range properties {
				propNames = append(propNames, propName)
			}
			sort.Strings(propNames)

			for _, propName := range propNames {
				propDef := properties[propName]
				propMap, ok := propDef.(map[string]interface{})
				if !ok {
					continue
				}

				fieldName := convertToGoFieldName(propName, acronyms)

				propType := determineGoType(propMap, definitions)

				propDefStr := fmt.Sprintf("\t%s %s `json:\"%s\"`\n", fieldName, propType, propName)
				if _, err := outputFile.WriteString(propDefStr); err != nil {
					return err
				}
			}
		}

		if _, err := outputFile.WriteString("}\n\n"); err != nil {
			return err
		}
	}

	cmd := exec.Command("go", "fmt", destination)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: Failed to format %s: %v\n", destination, err)
	}

	return nil
}

// convertToGoFieldName converts a JSON property name to a properly capitalized Go field name
func convertToGoFieldName(name string, acronyms map[string]bool) string {
	if name == "_meta" {
		return "Meta"
	}

	parts := strings.Split(name, "_")
	for i, part := range parts {
		lowerPart := strings.ToLower(part)
		if acronyms[lowerPart] {
			parts[i] = strings.ToUpper(lowerPart)
		} else {
			parts[i] = cases.Title(language.English).String(lowerPart)
		}
	}

	return strings.Join(parts, "")
}

// determineGoType determines the Go type for a JSON schema property
func determineGoType(propMap map[string]interface{}, definitions map[string]interface{}) string {
	if ref, ok := propMap["$ref"].(string); ok {
		parts := strings.Split(ref, "/")
		refType := parts[len(parts)-1]
		return refType
	}

	if propType, ok := propMap["type"].(string); ok && propType == "array" {
		if items, ok := propMap["items"].(map[string]interface{}); ok {
			itemType := determineGoType(items, definitions)
			return "[]" + itemType
		}
		return "[]interface{}"
	}

	if propType, ok := propMap["type"].(string); ok {
		format := ""
		if fmt, ok := propMap["format"].(string); ok {
			format = fmt
		}

		switch propType {
		case "string":
			if format == "date-time" {
				return "time.Time"
			}
			return "string"
		case "integer":
			if format == "int64" {
				return "int64"
			}
			return "int"
		case "number":
			return "float64"
		case "boolean":
			return "bool"
		case "object":
			return "map[string]interface{}"
		}
	}

	return "interface{}"
}

func generateType(prop openapi.Property) string {
	if prop.Ref != "" {
		parts := strings.Split(prop.Ref, "/")
		return parts[len(parts)-1]
	}

	if prop.Type == "array" && prop.Items != nil {
		return "[]" + generateType(*prop.Items)
	}

	if prop.AdditionalProperties != nil && *prop.AdditionalProperties {
		return "map[string]interface{}"
	}

	switch prop.Type {
	case "string":
		if len(prop.Enum) > 0 {
			if prop.Name != "" {
				parts := strings.Split(prop.Name, "_")
				for i, part := range parts {
					parts[i] = cases.Title(language.English).String(strings.ToLower(part))
				}
				return strings.Join(parts, "")
			}
		}
		if prop.Format == "date-time" {
			return "time.Time"
		}
		if prop.Format == "binary" {
			return "[]byte"
		}
		return "string"
	case "number":
		switch prop.Format {
		case "float32":
			return "float32"
		case "double":
			return "float64"
		default:
			return "float64"
		}
	case "integer":
		switch prop.Format {
		case "int32":
			return "int32"
		case "int64":
			return "int64"
		case "uint32":
			return "uint32"
		case "uint64":
			return "uint64"
		default:
			return "int"
		}
	case "boolean":
		return "bool"
	case "object":
		if len(prop.Properties) > 0 {
			return "struct{}"
		}
		return "map[string]interface{}"
	default:
		return "interface{}"
	}
}

// generateTag generates the JSON tag for a struct field
func generateTag(field string, prop openapi.Property, requiredFields []string) template.HTML {
	var tags []string

	jsonTag := fmt.Sprintf(`json:"%s`, field)
	isRequired := false
	for _, rf := range requiredFields {
		if rf == field {
			isRequired = true
			break
		}
	}

	if !isRequired {
		jsonTag += `,omitempty`
	}
	jsonTag += `"`

	tags = append(tags, jsonTag)

	if len(tags) > 0 {
		return template.HTML("`" + strings.Join(tags, " ") + "`")
	}

	return template.HTML("")
}

// formatDescription formats a description string as proper Go comments
// with each line prefixed by "// "
func formatDescription(description string) string {
	if description == "" {
		return ""
	}

	lines := strings.Split(description, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			lines[i] = "// " + line
		} else {
			lines[i] = "//"
		}
	}

	return strings.Join(lines, "\n")
}
