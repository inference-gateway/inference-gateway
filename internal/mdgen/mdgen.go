package mdgen

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/inference-gateway/inference-gateway/internal/openapi"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func GenerateConfigurationsMD(filePath string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	tmpl := `# Inference Gateway Configuration

{{- range $section := .Sections }}
{{- range $name, $section := $section }}
{{- if not (eq $section.Title "Providers") }}
## {{ $section.Title }}

| Environment Variable | Default Value | Description |
|---------------------|---------------|-------------|
{{- range $setting := $section.Settings }}
{{- range $field := $setting }}
| {{ $field.Env }} | ` + "`{{ if $field.Default }}{{ $field.Default }}{{ else }}\"\"{{ end }}`" + ` | {{ $field.Description }} |
{{- end }}
{{- end }}

{{- end }}
{{- end }}
{{- end }}
## API URLs and keys

| Environment Variable | Default Value | Description |
|---------------------|---------------|-------------|
{{- range $name, $provider := .Providers }}
| {{ upper $name }}_API_URL | ` + "`{{ $provider.URL }}`" + ` | The URL for {{ title $name }} API |
{{- end }}
`

	// Create template with functions
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
		"title": cases.Title(language.English).String,
	}

	t, err := template.New("configurations").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create file
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
