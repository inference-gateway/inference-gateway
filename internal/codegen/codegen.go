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
		"fieldType": func(env string, fieldType string) string {
			if strings.HasPrefix(env, "ENABLE_") {
				return "bool"
			}
			if strings.HasSuffix(env, "_TIMEOUT") {
				return "time.Duration"
			}
			return "string"
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
    {{- range $setting := $section.Settings }}
    {{- range $field := $setting }}
    {{ pascalCase $field.Env }} {{ fieldType $field.Env "string" }} ` + "`env:\"{{ $field.Env }}{{if $field.Default}}, default={{$field.Default}}{{end}}\" description:\"{{$field.Description}}\"`" + `
    {{- end }}
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
    {{- range $setting := $section.Settings }}
    {{- range $field := $setting }}
    {{ pascalCase (trimPrefix $field.Env "OIDC_") }} string ` + "`env:\"{{ trimPrefix $field.Env \"OIDC_\" }}{{if $field.Default}}, default={{$field.Default}}{{end}}\"{{if $field.Secret}} type:\"secret\"{{end}} description:\"{{$field.Description}}\"`" + `
    {{- end }}
    {{- end }}
}
{{- else if eq $name "server" }}

// Server configuration
type ServerConfig struct {
    {{- range $setting := $section.Settings }}
    {{- range $field := $setting }}
    {{ pascalCase (trimPrefix $field.Env "SERVER_") }} {{ fieldType $field.Env "string" }} ` + "`env:\"{{ trimPrefix $field.Env \"SERVER_\" }}{{if $field.Default}}, default={{$field.Default}}{{end}}\" description:\"{{$field.Description}}\"`" + `
    {{- end }}
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

import "fmt"

// Base provider configuration
type Config struct {
	ID           string
	Name         string
	URL          string
	Token        string
	AuthType     string
	ExtraHeaders map[string][]string
	Endpoints    struct {
		List     string
		Generate string
	}
}

// GetProviders returns a list of providers
func GetProviders(cfg map[string]*Config) []Provider {
	providerList := make([]Provider, 0, len(cfg))
	for _, provider := range cfg {
		providerList = append(providerList, &ProviderImpl{
			ID:           provider.ID,
			Name:         provider.Name,
			URL:          provider.URL,
			Token:        provider.Token,
			AuthType:     provider.AuthType,
			ExtraHeaders: provider.ExtraHeaders,
		})
	}
	return providerList
}

// GetProvider returns a provider by id
func GetProvider(cfg map[string]*Config, id string) (Provider, error) {
	provider, ok := cfg[id]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", id)
	}
	return &ProviderImpl{
		ID:           provider.ID,
		Name:         provider.Name,
		URL:          provider.URL,
		Token:        provider.Token,
		AuthType:     provider.AuthType,
		ExtraHeaders: provider.ExtraHeaders,
	}, nil
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

func generateProviderFile(destination, name string, config openapi.ProviderConfig) error {
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

func generateType(field openapi.SchemaField) string {
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
