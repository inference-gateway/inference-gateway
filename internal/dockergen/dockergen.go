package dockergen

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	openapi "github.com/inference-gateway/inference-gateway/internal/openapi"
)

const (
	serverHostEnv      = "SERVER_HOST"
	serverHostInDocker = "0.0.0.0"
)

func GenerateEnvExample(output string, oas string) error {
	// Read OpenAPI spec
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	tmpl := `{{- range $section := .Sections }}
{{- range $name, $section := $section }}
{{- if eq $name "providers" }}{{ else }}
# {{ $section.Title }}
{{- range $setting := $section.Settings }}
{{ $setting.Env }}={{ if eq $setting.Env $.ServerHostEnv }}{{ $.ServerHostInDocker }}{{ else if $setting.Default }}{{ $setting.Default }}{{ end }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
{{ if .Providers }}
# Providers
{{- range $name, $config := .Providers }}
{{ upper $name }}_API_KEY=
{{- end }}
{{- end }}
`

	t, err := template.New("env").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	}).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	f, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	data := struct {
		Sections           []map[string]openapi.Section
		Providers          map[string]openapi.ProviderConfig
		ServerHostEnv      string
		ServerHostInDocker string
	}{
		Sections:           schema.Components.Schemas.Config.XConfig.Sections,
		Providers:          schema.Components.Schemas.Provider.XProviderConfigs,
		ServerHostEnv:      serverHostEnv,
		ServerHostInDocker: serverHostInDocker,
	}

	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
