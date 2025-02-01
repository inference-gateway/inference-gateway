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
	"gopkg.in/yaml.v3"
)

// GenerateProviders generates providers Request Response schemas from an OpenAPI spec
func GenerateProviders(output string, openapiPath string) error {
	// Read OpenAPI spec
	data, err := os.ReadFile(openapiPath)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	var schema openapi.OpenAPISchema
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

	tmpl := template.Must(template.New("config").Funcs(funcMap).Parse(`package config
	
import (
	"context"
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
	{{- else if eq $name "oidc" }}
	// OIDC settings
	OIDC *OIDC ` + "`env:\", prefix=OIDC_\" description:\"OIDC configuration\"`" + `
	{{- else if eq $name "server" }}
	// Server settings
	Server *ServerConfig ` + "`env:\", prefix=SERVER_\" description:\"Server configuration\"`" + `
	{{- end }}
	{{- end }}
	{{- end }}

	// Providers map
	Providers map[string]*providers.Config
}

{{- range $section := .Sections }}
{{- range $name, $section := $section }}
{{- if eq $name "oidc" }}

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
		cfg.Providers = make(map[string]*providers.Config)
	}

	// Set defaults for each provider
	for id, defaults := range providers.Registry {
		if _, exists := cfg.Providers[id]; !exists {
			providerCfg := defaults
			url, ok := lookuper.Lookup(strings.ToUpper(id) + "_API_URL")
			if ok {
				providerCfg.URL = url
			}

			token, ok := lookuper.Lookup(strings.ToUpper(id) + "_API_KEY")
			if !ok {
				println("Warn: provider " + id + " is not configured")
			}
			providerCfg.Token = token
			cfg.Providers[id] = &providerCfg
		}
	}

	return *cfg, nil
}`))

	data := struct {
		Sections  []map[string]openapi.Section
		Providers map[string]openapi.ProviderConfig
	}{
		Sections:  schema.Components.Schemas.Config.XConfig.Sections,
		Providers: schema.Components.Schemas.Providers.XProviderConfigs,
	}

	f, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	// Format generated code
	cmd := exec.Command("go", "fmt", destination)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to format %s: %w", destination, err)
	}

	return nil
}

// GenerateProvidersRegistry generates a registry of all providers from OpenAPI Spec
func GenerateProvidersRegistry(destination string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		fmt.Printf("Error reading OpenAPI spec: %v\n", err)
		os.Exit(1)
	}

	providers := schema.Components.Schemas.Providers.XProviderConfigs

	caser := cases.Title(language.English)

	funcMap := template.FuncMap{
		"title": caser.String,
	}

	tmpl := template.Must(template.New("registry").
		Funcs(funcMap).
		Parse(`package providers

// Endpoints exposed by each provider
type Endpoints struct {
	List     string
	Generate string
}

// Base provider configuration
type Config struct {
	ID           string
	Name         string
	URL          string
	Token        string
	AuthType     string
	ExtraHeaders map[string][]string
	Endpoints    Endpoints
}

// The registry of all providers
var Registry = map[string]Config{
	{{- range $name, $config := .Providers}}
	{{title $name}}ID: {
		ID:       {{title $name}}ID,
		Name:     {{title $name}}DisplayName,
		URL:      {{title $name}}DefaultBaseURL,
		AuthType: AuthType{{title $config.AuthType}},
		{{- if $config.ExtraHeaders}}
		ExtraHeaders: map[string][]string{
			{{- range $key, $header := $config.ExtraHeaders}}
			"{{$key}}": {"{{index $header.Values 0}}"},
			{{- end}}
		},
		{{- end}}
	},
	{{- end}}
}`))

	data := struct {
		Providers map[string]openapi.ProviderConfig
	}{
		Providers: providers,
	}

	f, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	// Run go fmt on the generated file
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
		"title":        caser.String,
		"generateType": generateType,
		"hasPrefix":    strings.HasPrefix,
	}

	tmpl := template.Must(template.New("common").
		Funcs(funcMap).
		Parse(`package providers

import "time"

// The authentication type of the specific provider
const (
	{{- range $type := .Schemas.AuthType.Enum }}
	AuthType{{title .}} = "{{.}}"
	{{- end }}
)

// The default base URLs of each provider
const (
	{{- range $name, $config := .Providers }}
	{{title $name}}DefaultBaseURL = "{{$config.URL}}"
	{{- end }}
)

// The ID's of each provider
const (
	{{- range $name, $config := .Providers }}
	{{title $name}}ID = "{{$config.ID}}"
	{{- end }}
)

// Display names for providers
const (
	{{- range $name, $config := .Providers }}  
	{{title $name}}DisplayName = "{{title $name}}"
	{{- end }}
)

// Common response and request types
{{- range $name, $schema := .Schemas }}
type {{$name}} struct {
	{{- range $field, $prop := $schema.Properties }}
	{{title $field}} {{generateType $prop}} ` + "`json:\"{{$field}}\"`" + `
	{{- end }}
}

{{- end }}
`))

	data := struct {
		Providers map[string]openapi.ProviderConfig
		Schemas   map[string]openapi.SchemaProperty
	}{
		Providers: schema.Components.Schemas.Providers.XProviderConfigs,
		Schemas:   openapi.GetSchemas(schema),
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

	const clientTemplate = `package providers

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
    fullURL := c.scheme + "://" + c.hostname + ":" + c.port + url
    return c.client.Get(fullURL)
}

func (c *ClientImpl) Post(url string, bodyType string, body string) (*http.Response, error) {
    fullURL := c.scheme + "://" + c.hostname + ":" + c.port + url
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

	// Format generated code
	cmd := exec.Command("go", "fmt", destination)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to format %s: %w", destination, err)
	}

	return nil
}

func generateProviderFile(destination, name string, config openapi.ProviderConfig) error {
	caser := cases.Title(language.English)

	funcMap := template.FuncMap{
		"title":        caser.String,
		"generateType": generateType,
		"hasPrefix":    strings.HasPrefix,
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
type ListModelsResponse{{title $.Name}} struct {
    {{- if eq .Type "object" }}
    {{- range $key, $prop := .Properties }}
	{{- if not (hasPrefix $key "x-") }}
    {{title $key}} {{generateType $prop}} ` + "`json:\"{{$key}}\"`" + `
    {{- end }}
    {{- end }}
}
{{end}}

// Transform converts provider-specific response to common format
func (r *ListModelsResponse{{title $.Name}}) Transform() ListModelsResponse {
    {{- with .XTransform }}
    var models []map[string]interface{}
    {{- range .Mapping.Models.Transform }}
    for _, model := range r.Models {
        models = append(models, map[string]interface{}{
            {{- range . }}
            "{{.Target}}": {{if .Source}}model.{{.Source}}{{else}}"{{.Constant}}"{{end}},
            {{- end }}
        })
    }
    return ListModelsResponse{
        Provider: {{.Mapping.Provider}},
        Models:   models,
    }
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
		Config openapi.ProviderConfig
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

func generateType(prop openapi.Property) string {
	// Handle references first
	if prop.Ref != "" {
		parts := strings.Split(prop.Ref, "/")
		return parts[len(parts)-1]
	}

	// Handle arrays
	if prop.Type == "array" && prop.Items != nil {
		return "[]" + generateType(*prop.Items)
	}

	// Map basic types
	switch prop.Type {
	case "string":
		if prop.Format == "date-time" {
			return "time.Duration"
		}
		return "string"
	case "number":
		return "float64"
	case "integer":
		return "int"
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
