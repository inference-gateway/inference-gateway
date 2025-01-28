package dockergen

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/inference-gateway/inference-gateway/internal/openapi"
)

func GenerateEnvExample(output string, oas string) error {
	// Read OpenAPI spec
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	tmpl := `{{- range $section := .Sections }}
{{- range $name, $section := $section }}
# {{ $section.Title }}
{{- range $setting := $section.Settings }}
{{- range $field := $setting }}
{{- if not (eq $field.Env "{key}_API_URL") }}
{{- if not (eq $field.Env "{key}_API_KEY") }}
{{ $field.Env }}={{ if $field.Default }}{{ $field.Default }}{{ end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
# API URLs and keys
{{- range $name, $provider := .Providers }}
{{ upper $name }}_API_URL={{ $provider.URL }}
{{ upper $name }}_API_KEY=
{{- end }}
`

	// Create template with functions
	t, err := template.New("env").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	}).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create file
	f, err := os.Create(output)
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
