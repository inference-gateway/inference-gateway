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

func ProvidersClientConfig(destination, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	var clientSection openapi.Section
	for _, sectionMap := range schema.Components.Schemas.Config.XConfig.Sections {
		for name, section := range sectionMap {
			if name == "client" {
				clientSection = section
				break
			}
		}
	}

	if clientSection.Title == "" {
		return fmt.Errorf("client configuration not found in OpenAPI spec")
	}

	funcMap := template.FuncMap{
		"pascalCase": func(s string) string {
			parts := strings.Split(s, "_")
			for i, part := range parts {
				parts[i] = cases.Title(language.English).String(strings.ToLower(part))
			}
			return strings.Join(parts, "")
		},
	}

	const clientTemplate = `// Code generated from OpenAPI schema. DO NOT EDIT.
package client
{{- if .NeedsTime }}

import (
    "time"
)
{{- end }}

type ClientConfig struct {
    {{- range $setting := .ClientSettings }}
    {{ pascalCase $setting.Env }} {{ $setting.Type }} ` + "`" + `env:"{{ $setting.Env }}, default={{ $setting.Default }}" description:"{{ $setting.Description }}"` + "`" + `
    {{- end }}
}
`

	tmpl, err := template.New("client").Funcs(funcMap).Parse(clientTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	needsTime := false
	for _, setting := range clientSection.Settings {
		if strings.Contains(setting.Type, "time.") {
			needsTime = true
			break
		}
	}

	data := struct {
		ClientSettings []openapi.Setting
		NeedsTime      bool
	}{
		ClientSettings: clientSection.Settings,
		NeedsTime:      needsTime,
	}

	f, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	cmd := exec.Command("gofmt", "-w", destination)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to format %s: %w", destination, err)
	}

	return nil
}
