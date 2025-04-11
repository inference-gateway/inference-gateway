package kubegen

// Package kubegen provides functionality for generating Helm templates for Kubernetes
// ConfigMaps and Secrets from OpenAPI specifications.

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/inference-gateway/inference-gateway/internal/openapi"
)

// GenerateHelmSecret generates a Helm template for a Kubernetes Secret from an OpenAPI spec.
// The generated template uses Helm values (.Values.secrets) for configuration.
func GenerateHelmSecret(filePath string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	tmpl := `apiVersion: v1
kind: Secret
metadata:
  name: {{ "{{" }} .Values.envFrom.secretRef {{ "}}" }}
  labels:
    {{ "{{-" }} include "inference-gateway.labels" . | nindent 4 {{ "}}" }}
stringData:
  {{- range $section := .Sections }}
  {{- range $name, $section := $section }}
  {{- if or (eq $name "oidc") (eq $name "providers") }}
  # {{ $section.Title }}
  {{- range $setting := $section.Settings }}
  {{- if $setting.Secret }}
  {{ $setting.Env }}: ""
  {{- end }}
  {{- end }}
  {{- end -}}
  {{- end -}}
  {{- end }}
`

	t, err := template.New("helm-secret").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	}).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	data := struct {
		Sections  []map[string]openapi.Section
		Providers map[string]openapi.ProviderConfig
	}{
		Sections:  schema.Components.Schemas.Config.XConfig.Sections,
		Providers: schema.Components.Schemas.Provider.XProviderConfigs,
	}

	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func GenerateHelmConfigMap(filePath string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	tmpl := `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ "{{" }} .Values.envFrom.configMapRef {{ "}}" }}
  labels:
    {{ "{{-" }} include "inference-gateway.labels" . | nindent 4 {{ "}}" }}
data:
  {{- range $section := .Sections }}
  {{- range $name, $section := $section }}
  # {{ $section.Title }}
  {{- range $setting := $section.Settings }}
  {{- if not $setting.Secret }}
  {{ printf "{{- if .Values.config.%s }}" $setting.Env }}
  {{ $setting.Env }}: {{ printf "{{ .Values.config.%s | quote }}" $setting.Env }}
  {{ "{{- end }}" }}
  {{- end }}
  {{- end }}
  {{- end -}}
  {{- end }}
`

	t, err := template.New("helm-configmap").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	}).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	data := struct {
		Sections  []map[string]openapi.Section
		Providers map[string]openapi.ProviderConfig
	}{
		Sections:  schema.Components.Schemas.Config.XConfig.Sections,
		Providers: schema.Components.Schemas.Provider.XProviderConfigs,
	}

	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
