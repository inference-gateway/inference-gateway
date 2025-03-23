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
  {{- if or (eq $name "oidc") (eq $name "providers") }}
  # {{ $section.Title }}
  {{- range $setting := $section.Settings }}
  {{- if $setting.Secret }}
  {{ $setting.Env }}: "{{ if $setting.Default }}{{ $setting.Default }}{{ end }}"
  {{- end }}
  {{- end }}
  {{- end }}
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
		Providers: schema.Components.Schemas.Provider.XProviderConfigs,
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
  {{- if not $setting.Secret }}
  {{ $setting.Env }}: "{{ if $setting.Default }}{{ $setting.Default }}{{ end }}"
  {{- end }}
  {{- end }}
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
		Providers: schema.Components.Schemas.Provider.XProviderConfigs,
	}

	// Execute template
	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
