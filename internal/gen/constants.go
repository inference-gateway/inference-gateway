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

// Constants generates provider constants and enums from OpenAPI spec
func Constants(destination string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	funcMap := template.FuncMap{
		"pascalCase": func(s string) string {
			if strings.ToLower(s) == "id" {
				return "ID"
			}

			parts := strings.Split(s, "_")
			for i, part := range parts {
				parts[i] = cases.Title(language.English).String(strings.ToLower(part))
			}
			return strings.Join(parts, "")
		},
	}

	tmpl := template.Must(template.New("constants").
		Funcs(funcMap).
		Parse(`// Code generated from OpenAPI schema. DO NOT EDIT.
package constants

import (
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// The default base URLs of each provider
const (
    {{- range $name, $config := .Providers }}
    {{pascalCase $name}}DefaultBaseURL = "{{$config.URL}}"
    {{- end }}
)

// The default endpoints of each provider
const (
    {{- range $name, $config := .Providers }}
    {{pascalCase $name}}ModelsEndpoint = "{{(index $config.Endpoints "models").Endpoint}}"
    {{pascalCase $name}}ChatEndpoint   = "{{(index $config.Endpoints "chat").Endpoint}}"
    {{- with (index $config.Endpoints "responses").Endpoint }}
    {{pascalCase $name}}ResponsesEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "decisions").Endpoint }}
    {{pascalCase $name}}DecisionsEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "images").Endpoint }}
    {{pascalCase $name}}ImagesEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "images_edits").Endpoint }}
    {{pascalCase $name}}ImagesEditsEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "images_retrieve").Endpoint }}
    {{pascalCase $name}}ImagesRetrieveEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "speech").Endpoint }}
    {{pascalCase $name}}SpeechEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "sfx").Endpoint }}
    {{pascalCase $name}}SFXEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "sfx_video").Endpoint }}
    {{pascalCase $name}}SFXVideoEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "assets").Endpoint }}
    {{pascalCase $name}}AssetsEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "music").Endpoint }}
    {{pascalCase $name}}MusicEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "videos").Endpoint }}
    {{pascalCase $name}}VideosEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "videos_retrieve").Endpoint }}
    {{pascalCase $name}}VideosRetrieveEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "voices").Endpoint }}
    {{pascalCase $name}}VoicesEndpoint = "{{.}}"
    {{- end }}
    {{- with (index $config.Endpoints "voices_create").Endpoint }}
    {{pascalCase $name}}VoicesCreateEndpoint = "{{.}}"
    {{- end }}
    {{- end }}
)

// The ID's of each provider
const (
    {{- range $name, $config := .Providers }}
    {{pascalCase $name}}ID types.Provider = "{{$config.ID}}"
    {{- end }}
)

// Display names for providers
const (
    {{- range $name, $config := .Providers }}
    {{pascalCase $name}}DisplayName = "{{pascalCase $name}}"
    {{- end }}
)
`))

	data := struct {
		Providers map[string]openapi.ProviderConfig
	}{
		Providers: schema.Components.Schemas.Provider.XProviderConfigs,
	}

	f, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(schema.Components.Schemas.Provider.XProviderConfigs) == 0 {
		return fmt.Errorf("no provider configurations found in OpenAPI spec")
	}

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	cmd := exec.Command("gofmt", "-w", destination)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to format %s: %w", destination, err)
	}

	return nil
}
