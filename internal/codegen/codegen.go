package codegen

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"strings"

	"github.com/inference-gateway/inference-gateway/internal/openapi"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
	"log"
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
	{{- else if eq $name "telemetry" }}
	// Telemetry settings
	Telemetry *TelemetryConfig ` + "`env:\", prefix=TELEMETRY_\" description:\"Telemetry configuration\"`" + `
	{{- else if eq $name "mcp" }}
	// MCP settings
	MCP *MCPConfig ` + "`env:\", prefix=MCP_\" description:\"MCP configuration\"`" + `
	{{- else if eq $name "auth" }}
	// Authentication settings
	Auth *AuthConfig ` + "`env:\", prefix=AUTH_\" description:\"Authentication configuration\"`" + `
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
{{- if eq $name "telemetry" }}

// Telemetry configuration
type TelemetryConfig struct {
	{{- range $field := $section.Settings }}
	{{ pascalCase (trimPrefix $field.Env "TELEMETRY_") }} {{ $field.Type }} ` + "`env:\"{{ trimPrefix $field.Env \"TELEMETRY_\" }}{{if $field.Default}}, default={{$field.Default}}{{end}}\" description:\"{{$field.Description}}\"`" + `
	{{- end }}
}
{{- else if eq $name "mcp" }}

// MCP configuration
type MCPConfig struct {
	{{- range $field := $section.Settings }}
	{{ pascalCase (trimPrefix $field.Env "MCP_") }} {{ $field.Type }} ` + "`env:\"{{ trimPrefix $field.Env \"MCP_\" }}{{if $field.Default}}, default={{$field.Default}}{{end}}\" description:\"{{$field.Description}}\"`" + `
	{{- end }}
}
{{- else if eq $name "auth" }}

// Authentication configuration
type AuthConfig struct {
	{{- range $field := $section.Settings }}
	{{ pascalCase (trimPrefix $field.Env "AUTH_") }} {{ $field.Type }} ` + "`env:\"{{ trimPrefix $field.Env \"AUTH_\" }}{{if $field.Default}}, default={{$field.Default}}{{end}}\"{{if $field.Secret}} type:\"secret\"{{end}} description:\"{{$field.Description}}\"`" + `
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
			if (!ok || token == "") && id != providers.OllamaID {
				t := time.Now().UTC().Format(time.RFC3339)
				log.SetFlags(0)
				log.Printf("{\"level\":\"notice\",\"timestamp\":\"%s\",\"caller\":\"config/config.go:103\",\"msg\":\"provider is not configured\",\"provider\":\"%s\"}", t, string(id))
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
        "Config{ApplicationName:%s, Version:%s Environment:%s, Telemetry:%+v, "+
            "MCP:%+v, Auth:%+v, Server:%+v, Client:%+v, Providers:%+v}",
        APPLICATION_NAME,
        VERSION,
        cfg.Environment,
        cfg.Telemetry,
        cfg.MCP,
        cfg.Auth,
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

//go:generate mockgen -source=client.go -destination=../tests/mocks/providers/client.go -package=providersmocks
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
    {{ pascalCase $setting.Env }} {{ $setting.Type }} ` + "`" + `env:"{{ $setting.Env }}, default={{ $setting.Default }}" description:"{{ $setting.Description }}"` + "`" + `
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
            ForceAttemptHTTP2:     true,
            DisableCompression:    cfg.ClientDisableCompression,
            ResponseHeaderTimeout: cfg.ClientResponseHeaderTimeout,
            ExpectContinueTimeout: cfg.ClientExpectContinueTimeout,
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

// readIgnoreFile reads the .openapi-ignore file and returns a set of ignored files
func readIgnoreFile(ignoreFilePath string) (map[string]bool, error) {
	ignored := make(map[string]bool)

	data, err := os.ReadFile(ignoreFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ignored, nil
		}
		return nil, fmt.Errorf("failed to read ignore file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			ignored[line] = true
		}
	}

	return ignored, nil
}

// GenerateProviders generates individual provider files based on their configuration
// Only generates files not listed in .openapi-ignore and uses only the OpenAI template
func GenerateProviders(outputDir string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	if len(schema.Components.Schemas.Provider.XProviderConfigs) == 0 {
		return fmt.Errorf("no provider configurations found in OpenAPI spec")
	}

	ignoreFilePath := ".openapi-ignore"
	ignored, err := readIgnoreFile(ignoreFilePath)
	if err != nil {
		return fmt.Errorf("failed to read ignore file: %w", err)
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
	}

	openaiCompatibleTemplate := `package providers

type ListModelsResponse{{.ProviderName}} struct {
	Object string  ` + "`json:\"object\"`" + `
	Data   []Model ` + "`json:\"data\"`" + `
}

func (l *ListModelsResponse{{.ProviderName}}) Transform() ListModelsResponse {
	provider := {{.ProviderName}}ID
	models := make([]Model, len(l.Data))
	for i, model := range l.Data {
		model.ServedBy = provider
		model.ID = string(provider) + "/" + model.ID
		models[i] = model
	}

	return ListModelsResponse{
		Provider: &provider,
		Object:   l.Object,
		Data:     models,
	}
}
`

	for providerName, config := range schema.Components.Schemas.Provider.XProviderConfigs {
		filename := fmt.Sprintf("%s.go", strings.ToLower(providerName))
		fullPath := fmt.Sprintf("%s/%s", outputDir, filename)

		relativePath := fmt.Sprintf("%s/%s", strings.TrimPrefix(outputDir, "./"), filename)

		if ignored[relativePath] {
			fmt.Printf("Skipping %s (found in .openapi-ignore)\n", relativePath)
			continue
		}

		tmpl, err := template.New(providerName).Funcs(funcMap).Parse(openaiCompatibleTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse template for %s: %w", providerName, err)
		}

		data := struct {
			ProviderName string
			Config       openapi.ProviderConfig
		}{
			ProviderName: caser.String(providerName),
			Config:       config,
		}

		f, err := os.Create(fullPath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", fullPath, err)
		}
		defer f.Close()

		if err := tmpl.Execute(f, data); err != nil {
			return fmt.Errorf("failed to execute template for %s: %w", providerName, err)
		}

		cmd := exec.Command("go", "fmt", fullPath)
		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: Failed to format %s: %v\n", fullPath, err)
		}

		fmt.Printf("Generated %s\n", fullPath)
	}

	return nil
}

// GenerateProviderRegistry generates the provider registry based on provider configurations
func GenerateProviderRegistry(destination string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	if len(schema.Components.Schemas.Provider.XProviderConfigs) == 0 {
		return fmt.Errorf("no provider configurations found in OpenAPI spec")
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
		"getAuthType": func(authType string) string {
			switch authType {
			case "bearer":
				return "AuthTypeBearer"
			case "xheader":
				return "AuthTypeXheader"
			case "query":
				return "AuthTypeQuery"
			case "none":
				return "AuthTypeNone"
			default:
				return "AuthTypeBearer"
			}
		},
	}

	registryTemplate := `// Code generated from OpenAPI schema. DO NOT EDIT.
package providers

import (
	"fmt"

	"github.com/inference-gateway/inference-gateway/logger"
)

// Base provider configuration
type Config struct {
	ID           Provider
	Name         string
	URL          string
	Token        string
	AuthType     string
	ExtraHeaders map[string][]string
	Endpoints    Endpoints
}

//go:generate mockgen -source=registry.go -destination=../tests/mocks/providers/registry.go -package=providersmocks
type ProviderRegistry interface {
	GetProviders() map[Provider]*Config
	BuildProvider(providerID Provider, client Client) (IProvider, error)
}

type ProviderRegistryImpl struct {
	cfg    map[Provider]*Config
	logger logger.Logger
}

func NewProviderRegistry(cfg map[Provider]*Config, logger logger.Logger) ProviderRegistry {
	return &ProviderRegistryImpl{
		cfg:    cfg,
		logger: logger,
	}
}

func (p *ProviderRegistryImpl) GetProviders() map[Provider]*Config {
	return p.cfg
}

func (p *ProviderRegistryImpl) BuildProvider(providerID Provider, client Client) (IProvider, error) {
	provider, ok := p.cfg[providerID]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", providerID)
	}

	if provider.AuthType != AuthTypeNone && provider.Token == "" {
		return nil, fmt.Errorf("provider %s token not configured", providerID)
	}

	return &ProviderImpl{
		id:           &provider.ID,
		name:         provider.Name,
		url:          provider.URL,
		token:        provider.Token,
		authType:     provider.AuthType,
		extraHeaders: provider.ExtraHeaders,
		endpoints:    provider.Endpoints,
		logger:       p.logger,
		client:       client,
	}, nil
}

// The registry of all providers
var Registry = map[Provider]*Config{
	{{- range $name, $config := .Providers }}
	{{title $name}}ID: {
		ID:       {{title $name}}ID,
		Name:     {{title $name}}DisplayName,
		URL:      {{title $name}}DefaultBaseURL,
		AuthType: {{getAuthType $config.AuthType}},
		{{- if eq $name "anthropic" }}
		ExtraHeaders: map[string][]string{
			"anthropic-version": {"2023-06-01"},
		},
		{{- end }}
		Endpoints: Endpoints{
			Models: {{title $name}}ModelsEndpoint,
			Chat:   {{title $name}}ChatEndpoint,
		},
	},
	{{- end }}
}
`

	tmpl, err := template.New("registry").Funcs(funcMap).Parse(registryTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse registry template: %w", err)
	}

	data := struct {
		Providers map[string]openapi.ProviderConfig
	}{
		Providers: schema.Components.Schemas.Provider.XProviderConfigs,
	}

	f, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create registry file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute registry template: %w", err)
	}

	cmd := exec.Command("go", "fmt", destination)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: Failed to format %s: %v\n", destination, err)
	}

	fmt.Printf("Generated provider registry: %s\n", destination)
	return nil
}
