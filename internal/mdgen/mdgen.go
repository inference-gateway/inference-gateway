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

	const mdTemplate = `## Configurations
{{- range $index, $sectionMap := .Sections }}
{{ range $name, $section := $sectionMap }}
### {{ $section.Title }}
| Environment Variable | Default Value | Description |
|---------------------|---------------|-------------|
{{- range $setting := $section.Settings }}
| {{ $setting.Env }} | ` + "`{{ if $setting.Default }}{{ $setting.Default }}{{ else }}\"\"{{ end }}`" + ` | {{ $setting.Description }} |
{{- end }}
{{ end }}
{{- end }}
`

	// Create template with functions
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
		"title": cases.Title(language.English).String,
	}

	t, err := template.New("configurations").Funcs(funcMap).Parse(mdTemplate)
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
