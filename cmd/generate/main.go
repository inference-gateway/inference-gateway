package main

import (
	"flag"
	"fmt"
	"html/template"
	"os/exec"
	"sort"

	"os"
	"strings"

	"github.com/inference-gateway/inference-gateway/internal/mdgen"
	"github.com/inference-gateway/inference-gateway/internal/openapi"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

var (
	output string
	_type  string
)

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
		// comments := parseStructComments("config.go", "Config")
		// generateEnvExample(output)
	case "ConfigMap":
		// comments := parseStructComments("config.go", "Config")
		// generateConfigMap(output)
	case "Secret":
		// comments := parseStructComments("config.go", "Config")
		// generateSecret(output)
	case "MD":
		// comments := parseStructComments("config.go", "Config")
		err := mdgen.GenerateConfigurationsMD(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating MD: %v\n", err)
			os.Exit(1)
		}
	case "Providers":
		if err := generateProviders(output, "openapi.yaml"); err != nil {
			fmt.Printf("Error generating providers: %v\n", err)
			os.Exit(1)
		}
	case "Config":
		err := generateConfig(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating config: %v\n", err)
			os.Exit(1)
		}
		err = generateProvidersRegistry("providers/registry.go", "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating providers registry: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Println("Invalid type specified")
		os.Exit(1)
	}
}

func generateProviders(output string, openapiPath string) error {
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

func generateConfig(destination string, oas string) error {
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
//go:generate go run ../cmd/generate/main.go -type=Secret -output=../examples/kubernetes/basic/inference-gateway/secret.yaml
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/hybrid/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=Secret -output=../examples/kubernetes/hybrid/inference-gateway/secret.yaml
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/authentication/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=Secret -output=../examples/kubernetes/authentication/inference-gateway/secret.yaml
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/agent/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=Secret -output=../examples/kubernetes/agent/inference-gateway/secret.yaml
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

func generateProvidersRegistry(destination string, oas string) error {
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

// func generateFromConfig(cfg *config.Config) map[string]FieldInfo {
// 	fields := make(map[string]FieldInfo)
// 	t := reflect.TypeOf(*cfg)

// 	// Process base struct fields
// 	for i := 0; i < t.NumField(); i++ {
// 		field := t.Field(i)
// 		tag := field.Tag.Get("env")
// 		desc := field.Tag.Get("description")
// 		if tag == "" {
// 			continue
// 		}

// 		// Parse env tag
// 		envName, defaultValue, isSecret := parseEnvTag(tag)
// 		fields[field.Name] = FieldInfo{
// 			EnvName:      envName,
// 			DefaultValue: defaultValue,
// 			Description:  desc,
// 			IsSecret:     isSecret,
// 		}

// 		// Handle nested structs (OIDC, Server, Provider configs)
// 		if field.Type.Kind() == reflect.Ptr {
// 			nestedFields := processNestedStruct(field)
// 			for k, v := range nestedFields {
// 				fields[k] = v
// 			}
// 		}
// 	}
// 	return fields
// }

// func processNestedStruct(field reflect.StructField) map[string]FieldInfo {
// 	fields := make(map[string]FieldInfo)

// 	structType := field.Type.Elem()
// 	prefix := ""

// 	// Extract clean prefix from env tag
// 	if tag := field.Tag.Get("env"); tag != "" {
// 		parts := strings.Split(tag, ",")
// 		for _, part := range parts {
// 			part = strings.TrimSpace(part)
// 			if strings.HasPrefix(part, "prefix=") {
// 				prefix = strings.Trim(strings.TrimPrefix(part, "prefix="), "\"")
// 				break
// 			}
// 		}
// 	}

// 	// Process nested struct fields
// 	for i := 0; i < structType.NumField(); i++ {
// 		nestedField := structType.Field(i)
// 		tag := nestedField.Tag.Get("env")
// 		desc := nestedField.Tag.Get("description")

// 		if tag == "" {
// 			continue
// 		}

// 		// Parse env tag without prefix=
// 		envName, defaultValue, isSecret := parseEnvTag(tag)

// 		// Add clean prefix to env name
// 		if prefix != "" {
// 			envName = prefix + envName
// 		}

// 		fields[nestedField.Name] = FieldInfo{
// 			EnvName:      envName,
// 			DefaultValue: defaultValue,
// 			Description:  desc,
// 			IsSecret:     isSecret,
// 		}

// 		// Handle nested struct fields recursively
// 		if nestedField.Type.Kind() == reflect.Ptr {
// 			nestedFields := processNestedStruct(nestedField)
// 			for k, v := range nestedFields {
// 				fields[k] = v
// 			}
// 		}
// 	}

// 	return fields
// }

// func generateEnvExample(filePath string) error {
// 	cfg := &config.Config{}
// 	fields := generateFromConfig(cfg)

// 	var sb strings.Builder
// 	for _, field := range fields {
// 		if field.Description != "" {
// 			sb.WriteString(fmt.Sprintf("# %s\n", field.Description))
// 		}
// 		sb.WriteString(fmt.Sprintf("%s=%s\n", field.EnvName, field.DefaultValue))
// 	}

// 	return os.WriteFile(filePath, []byte(sb.String()), 0644)
// }

// func generateConfigMap(filePath string) error {
// 	cfg := &config.Config{}
// 	fields := generateFromConfig(cfg)

// 	// Group fields by category
// 	general := make(map[string]FieldInfo)
// 	server := make(map[string]FieldInfo)
// 	api := make(map[string]FieldInfo)

// 	// Provider prefixes to ensure we catch all
// 	providers := []string{
// 		"OLLAMA_",
// 		"GROQ_",
// 		"OPENAI_",
// 		"GOOGLE_",
// 		"CLOUDFLARE_",
// 		"COHERE_",
// 		"ANTHROPIC_",
// 	}

// 	for _, field := range fields {
// 		if field.IsSecret {
// 			continue
// 		}

// 		// Check if field belongs to a provider
// 		isProviderField := false
// 		for _, prefix := range providers {
// 			if strings.HasPrefix(field.EnvName, prefix) {
// 				if strings.HasSuffix(field.EnvName, "_API_URL") {
// 					api[field.EnvName] = field
// 					isProviderField = true
// 					break
// 				}
// 			}
// 		}

// 		// If not a provider field, categorize normally
// 		if !isProviderField {
// 			switch {
// 			case strings.HasPrefix(field.EnvName, "SERVER_"):
// 				server[field.EnvName] = field
// 			case strings.HasPrefix(field.EnvName, "OIDC_"):
// 				if !strings.Contains(field.EnvName, "SECRET") {
// 					general[field.EnvName] = field
// 				}
// 			case !strings.Contains(field.EnvName, "API_KEY") &&
// 				!strings.Contains(field.EnvName, "TOKEN"):
// 				general[field.EnvName] = field
// 			}
// 		}
// 	}

// 	var sb strings.Builder
// 	sb.WriteString("---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n")
// 	sb.WriteString("  name: inference-gateway\n")
// 	sb.WriteString("  namespace: inference-gateway\n")
// 	sb.WriteString("  labels:\n    app: inference-gateway\n")
// 	sb.WriteString("data:\n")

// 	// Write general settings
// 	if len(general) > 0 {
// 		sb.WriteString("  # General settings\n")
// 		writeFields(&sb, general)
// 	}

// 	// Write server settings
// 	if len(server) > 0 {
// 		sb.WriteString("  # Server settings\n")
// 		writeFields(&sb, server)
// 	}

// 	// Write API URLs
// 	if len(api) > 0 {
// 		sb.WriteString("  # API URLs and keys\n")
// 		writeFields(&sb, api)
// 	}

// 	return os.WriteFile(filePath, []byte(sb.String()), 0644)
// }

func writeFields(sb *strings.Builder, fields map[string]FieldInfo) {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	for _, key := range keys {
		field := fields[key]
		if field.DefaultValue != "" {
			sb.WriteString(fmt.Sprintf("  %s: \"%s\"\n", field.EnvName, field.DefaultValue))
		} else {
			sb.WriteString(fmt.Sprintf("  %s: \"\"\n", field.EnvName))
		}
	}
}

// func generateSecret(filePath string) error {
// 	cfg := &config.Config{}
// 	fields := generateFromConfig(cfg)

// 	var sb strings.Builder
// 	sb.WriteString("apiVersion: v1\nkind: Secret\nmetadata:\n  name: inference-gateway\ntype: Opaque\nstringData:\n")

// 	for _, field := range fields {
// 		if field.IsSecret {
// 			if field.Description != "" {
// 				sb.WriteString(fmt.Sprintf("  # %s\n", field.Description))
// 			}
// 			sb.WriteString(fmt.Sprintf("  %s: \"\"\n", field.EnvName))
// 		}
// 	}

// 	return os.WriteFile(filePath, []byte(sb.String()), 0644)
// }

type FieldInfo struct {
	EnvName      string
	DefaultValue string
	Description  string
	IsSecret     bool
}

func parseEnvTag(tag string) (envName, defaultValue string, isSecret bool) {
	parts := strings.Split(tag, ",")
	envName = strings.TrimSpace(parts[0])

	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "default=") {
			defaultValue = strings.Trim(strings.TrimPrefix(part, "default="), "\"")
		}
		if strings.Contains(part, "type:\"secret\"") {
			isSecret = true
		}
	}

	return
}
