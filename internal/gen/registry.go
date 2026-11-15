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

// ProviderRegistry generates the provider registry based on provider configurations
func ProviderRegistry(destination string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	if len(schema.Components.Schemas.Provider.XProviderConfigs) == 0 {
		return fmt.Errorf("no provider configurations found in OpenAPI spec")
	}

	caser := cases.Title(language.English)

	funcMap := template.FuncMap{
		"title": caser.String,
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
		"getAuthType": func(authType string) string {
			switch authType {
			case "bearer":
				return "AuthTypeBearer"
			case "xheader":
				return "AuthTypeXheader"
			case "query":
				return "AuthTypeQuery"
			case "none":
				return "AuthTypeNone"
			default:
				return "AuthTypeBearer"
			}
		},
	}

	registryTemplate := `// Code generated from OpenAPI schema. DO NOT EDIT.
package registry

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// The registry of all providers
var Registry = map[types.Provider]*ProviderConfig{
	{{- range $name, $config := .Providers }}
	constants.{{pascalCase $name}}ID: {
		ID:             constants.{{pascalCase $name}}ID,
		Name:           constants.{{pascalCase $name}}DisplayName,
		URL:            constants.{{pascalCase $name}}DefaultBaseURL,
		AuthType:       constants.{{getAuthType $config.AuthType}},
		{{- with $config.AuthHeader }}
		AuthHeader:     "{{.}}",
		{{- end }}
		{{- if $config.ExtraHeaders }}
		ExtraHeaders: map[string][]string{
			{{- range $header, $value := $config.ExtraHeaders }}
			"{{ $header }}": { {{- range $i, $v := $value.Values }}{{ if $i }}, {{ end }}"{{ $v }}"{{- end }}},
			{{- end }}
		},
		{{- end }}
		Endpoints: types.Endpoints{
			Models: constants.{{pascalCase $name}}ModelsEndpoint,
			Chat:   constants.{{pascalCase $name}}ChatEndpoint,
			{{- if (index $config.Endpoints "responses").Endpoint }}
			Responses: ptr(constants.{{pascalCase $name}}ResponsesEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "decisions").Endpoint }}
			Decisions: ptr(constants.{{pascalCase $name}}DecisionsEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "images").Endpoint }}
			Images: ptr(constants.{{pascalCase $name}}ImagesEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "images_edits").Endpoint }}
			ImagesEdits: ptr(constants.{{pascalCase $name}}ImagesEditsEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "images_retrieve").Endpoint }}
			ImagesRetrieve: ptr(constants.{{pascalCase $name}}ImagesRetrieveEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "speech").Endpoint }}
			Speech: ptr(constants.{{pascalCase $name}}SpeechEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "sfx").Endpoint }}
			SFX: ptr(constants.{{pascalCase $name}}SFXEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "sfx_video").Endpoint }}
			SFXVideo: ptr(constants.{{pascalCase $name}}SFXVideoEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "assets").Endpoint }}
			Assets: ptr(constants.{{pascalCase $name}}AssetsEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "music").Endpoint }}
			Music: ptr(constants.{{pascalCase $name}}MusicEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "videos").Endpoint }}
			Videos: ptr(constants.{{pascalCase $name}}VideosEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "videos_retrieve").Endpoint }}
			VideosRetrieve: ptr(constants.{{pascalCase $name}}VideosRetrieveEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "voices").Endpoint }}
			Voices: ptr(constants.{{pascalCase $name}}VoicesEndpoint),
			{{- end }}
			{{- if (index $config.Endpoints "voices_create").Endpoint }}
			VoicesCreate: ptr(constants.{{pascalCase $name}}VoicesCreateEndpoint),
			{{- end }}
		},
	},
	{{- end }}
}
`

	tmpl, err := template.New("registry").Funcs(funcMap).Parse(registryTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse registry template: %w", err)
	}

	data := struct {
		Providers map[string]openapi.ProviderConfig
	}{
		Providers: schema.Components.Schemas.Provider.XProviderConfigs,
	}

	f, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create registry file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute registry template: %w", err)
	}

	cmd := exec.Command("gofmt", "-w", destination)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to format %s: %w", destination, err)
	}

	fmt.Printf("Generated provider registry: %s\n", destination)
	return nil
}
