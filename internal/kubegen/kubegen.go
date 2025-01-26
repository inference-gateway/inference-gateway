package kubegen

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/inference-gateway/inference-gateway/internal/openapi"
)

func GenerateSecret(filePath string, oas string) error {
	// Read OpenAPI spec
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	tmpl := `---
apiVersion: v1
kind: Secret
metadata:
name: inference-gateway
namespace: inference-gateway
labels:
app: inference-gateway
stringData:
  {{- range $section := .Sections }}
  {{- range $name, $section := $section }}
  {{- $hasSecrets := false }}
  {{- range $setting := $section.Settings }}
  {{- range $field := $setting }}
  {{- if $field.Secret }}
  {{- $hasSecrets = true }}
  {{- end }}
  {{- end }}
  {{- end }}
  {{- if $hasSecrets }}
  # {{ $section.Title }}
  {{- range $setting := $section.Settings }}
  {{- range $field := $setting }}
  {{- if $field.Secret }}
  {{- if not (eq $field.Env "{key}_API_KEY") }}
  {{ $field.Env }}: "{{ if $field.Default }}{{ $field.Default }}{{ end }}"
  {{- end }}
  {{- end }}
  {{- end }}
  {{- end }}
  {{- end }}
  {{- end }}
  {{- end }}
  {{- if .Providers }}
  {{- range $name, $provider := .Providers }}
  {{ upper $name }}_API_KEY: ""
  {{- end }}
  {{- end }}
`

	// Create template with functions
	t, err := template.New("configmap").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	}).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create file with proper indentation
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	// Prepare template data
	data := struct {
		Sections  []map[string]openapi.Section
		Providers map[string]openapi.ProviderConfig
	}{
		Sections:  schema.Components.Schemas.Config.XConfig.Sections,
		Providers: schema.Components.Schemas.Providers.XProviderConfigs,
	}

	// Execute template
	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func GenerateConfigMap(filePath string, oas string) error {
	// Read OpenAPI spec
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	tmpl := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: inference-gateway
  namespace: inference-gateway
  labels:
    app: inference-gateway
data:
  {{- range $section := .Sections }}
  {{- range $name, $section := $section }}
  # {{ $section.Title }}
  {{- range $setting := $section.Settings }}
  {{- range $field := $setting }}
  {{- if not $field.Secret }}
  {{- if not (eq $field.Env "{key}_API_URL") }}
  {{ $field.Env }}: "{{ if $field.Default }}{{ $field.Default }}{{ end }}"
  {{- end }}
  {{- end }}
  {{- end }}
  {{- end }}
  {{- end }}
  {{- end }}
  {{- if .Providers }}
  {{- range $name, $provider := .Providers }}
  {{ upper $name }}_API_URL: {{ $provider.URL }}
  {{- end }}
  {{- end }}
`

	// Create template with functions
	t, err := template.New("configmap").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	}).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create file with proper indentation
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	// Prepare template data
	data := struct {
		Sections  []map[string]openapi.Section
		Providers map[string]openapi.ProviderConfig
	}{
		Sections:  schema.Components.Schemas.Config.XConfig.Sections,
		Providers: schema.Components.Schemas.Providers.XProviderConfigs,
	}

	// Execute template
	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
