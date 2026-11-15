// Package gen holds the build-time generators behind cmd/generate. The OpenAPI
// spec produces generated Go code and Markdown; models.go instead syncs the
// models.dev metadata table from the models.dev dataset.
package gen

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"

	cases "golang.org/x/text/cases"
	language "golang.org/x/text/language"

	openapi "github.com/inference-gateway/inference-gateway/internal/openapi"
)

// Config generates a configuration file from an OpenAPI spec
func Config(destination string, oas string) error {
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
	"time"

	client "github.com/inference-gateway/inference-gateway/providers/client"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
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
	{{- else if eq $name "a2a" }}
	// A2A settings
	A2A *A2AConfig ` + "`env:\", prefix=A2A_\" description:\"A2A configuration\"`" + `
	{{- else if eq $name "guardrails" }}
	// Guardrails settings
	Guardrails *GuardrailsConfig ` + "`env:\", prefix=GUARDRAILS_\" description:\"Guardrails configuration\"`" + `
	{{- else if eq $name "auth" }}
	// Authentication settings
	Auth *AuthConfig ` + "`env:\", prefix=AUTH_\" description:\"Authentication configuration\"`" + `
	{{- else if eq $name "server" }}
	// Server settings
	Server *ServerConfig ` + "`env:\", prefix=SERVER_\" description:\"Server configuration\"`" + `
	{{- else if eq $name "routing" }}
	// Routing settings
	Routing *RoutingConfig ` + "`env:\", prefix=ROUTING_\" description:\"Routing configuration\"`" + `
	{{- else if eq $name "client" }}
	// Client settings
	Client *client.ClientConfig ` + "`description:\"Client configuration\"`" + `
	{{- end }}
	{{- end }}
	{{- end }}

	// Providers map
	Providers map[types.Provider]*registry.ProviderConfig
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
{{- else if eq $name "a2a" }}

// A2A configuration
type A2AConfig struct {
	{{- range $field := $section.Settings }}
	{{ pascalCase (trimPrefix $field.Env "A2A_") }} {{ $field.Type }} ` + "`env:\"{{ trimPrefix $field.Env \"A2A_\" }}{{if $field.Default}}, default={{$field.Default}}{{end}}\" description:\"{{$field.Description}}\"`" + `
	{{- end }}
}
{{- else if eq $name "guardrails" }}

// Guardrails configuration
type GuardrailsConfig struct {
	{{- range $field := $section.Settings }}
	{{ pascalCase (trimPrefix $field.Env "GUARDRAILS_") }} {{ $field.Type }} ` + "`env:\"{{ trimPrefix $field.Env \"GUARDRAILS_\" }}{{if $field.Default}}, default={{$field.Default}}{{end}}\" description:\"{{$field.Description}}\"`" + `
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
{{- else if eq $name "routing" }}

// Routing configuration
type RoutingConfig struct {
	{{- range $field := $section.Settings }}
	{{ pascalCase (trimPrefix $field.Env "ROUTING_") }} {{ $field.Type }} ` + "`env:\"{{ trimPrefix $field.Env \"ROUTING_\" }}{{if $field.Default}}, default={{$field.Default}}{{end}}\" description:\"{{$field.Description}}\"`" + `
	{{- end }}
}
{{- end }}
{{- end }}
{{- end }}
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

	cmd := exec.Command("gofmt", "-w", destination)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to format %s: %w", destination, err)
	}

	return nil
}
