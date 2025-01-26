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

func GenerateConfig(destination string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		fmt.Printf("Error reading OpenAPI spec: %v\n", err)
		os.Exit(1)
	}

	providers := schema.Components.Schemas.Providers.XProviderConfigs

	caser := cases.Title(language.English)

	funcMap := template.FuncMap{
		"title": caser.String,
		"upper": strings.ToUpper,
	}

	tmpl := template.Must(template.New("config").
		Funcs(funcMap).
		Parse(`package config

import (
    "context"
	"fmt"
    "strings"
    "time"

    "github.com/inference-gateway/inference-gateway/providers"
    "github.com/sethvargo/go-envconfig"
)

// Config holds the configuration for the Inference Gateway.
//
//go:generate go run ../cmd/generate/main.go -type=Env -output=../examples/docker-compose/.env.example
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/basic/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/hybrid/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/authentication/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/agent/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=MD -output=../Configurations.md
type Config struct {
    // General settings
    ApplicationName string ` + "`env:\"APPLICATION_NAME, default=inference-gateway\" description:\"The name of the application\"`" + `
    Environment    string ` + "`env:\"ENVIRONMENT, default=production\" description:\"The environment\"`" + `
    EnableTelemetry bool   ` + "`env:\"ENABLE_TELEMETRY, default=false\" description:\"Enable telemetry\"`" + `
    EnableAuth     bool   ` + "`env:\"ENABLE_AUTH, default=false\" description:\"Enable authentication\"`" + `

    // Auth settings
    OIDC *OIDC ` + "`env:\", prefix=OIDC_\" description:\"OIDC configuration\"`" + `

    // Server settings
    Server *ServerConfig ` + "`env:\", prefix=SERVER_\" description:\"Server configuration\"`" + `

    // Providers map
    Providers map[string]*providers.Config
}

// OIDC configuration
type OIDC struct {
    IssuerURL    string ` + "`env:\"ISSUER_URL, default=http://keycloak:8080/realms/inference-gateway-realm\" description:\"OIDC issuer URL\"`" + `
    ClientID     string ` + "`env:\"CLIENT_ID, default=inference-gateway-client\" type:\"secret\" description:\"OIDC client ID\"`" + `
    ClientSecret string ` + "`env:\"CLIENT_SECRET\" type:\"secret\" description:\"OIDC client secret\"`" + `
}

// Server configuration
type ServerConfig struct {
    Host         string        ` + "`env:\"HOST, default=0.0.0.0\" description:\"Server host\"`" + `
    Port         string        ` + "`env:\"PORT, default=8080\" description:\"Server port\"`" + `
    ReadTimeout  time.Duration ` + "`env:\"READ_TIMEOUT, default=30s\" description:\"Read timeout\"`" + `
    WriteTimeout time.Duration ` + "`env:\"WRITE_TIMEOUT, default=30s\" description:\"Write timeout\"`" + `
    IdleTimeout  time.Duration ` + "`env:\"IDLE_TIMEOUT, default=120s\" description:\"Idle timeout\"`" + `
    TLSCertPath  string        ` + "`env:\"TLS_CERT_PATH\" description:\"TLS certificate path\"`" + `
    TLSKeyPath   string        ` + "`env:\"TLS_KEY_PATH\" description:\"TLS key path\"`" + `
}

// GetProviders returns a list of providers
func (c *Config) GetProviders() []providers.Provider {
    providerList := make([]providers.Provider, 0, len(c.Providers))
    for _, provider := range c.Providers {
        providerList = append(providerList, &providers.ProviderImpl{
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
func (c *Config) GetProvider(id string) (providers.Provider, error) {
	provider, ok := c.Providers[id]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", id)
	}
	return &providers.ProviderImpl{
		ID:           provider.ID,
		Name:         provider.Name,
		URL:          provider.URL,
		Token:        provider.Token,
		AuthType:     provider.AuthType,
		ExtraHeaders: provider.ExtraHeaders,
	}, nil
}

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
}
`))

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
