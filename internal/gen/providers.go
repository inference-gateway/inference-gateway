package gen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	cases "golang.org/x/text/cases"
	language "golang.org/x/text/language"

	openapi "github.com/inference-gateway/inference-gateway/internal/openapi"
)

// readIgnoreFile reads the .openapi-ignore file and returns a set of ignored files
func readIgnoreFile(ignoreFilePath string) (map[string]bool, error) {
	ignored := make(map[string]bool)

	data, err := os.ReadFile(ignoreFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ignored, nil
		}
		return nil, fmt.Errorf("failed to read ignore file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			ignored[line] = true
		}
	}

	return ignored, nil
}

const generatedHeader = "// Code generated from OpenAPI schema. DO NOT EDIT."

// removeStaleTransformers deletes generated transformer files whose provider
// is no longer in the schema, so dropping a provider upstream cannot leave a
// file behind that references constants which no longer exist.
func removeStaleTransformers(outputDir string, providers map[string]openapi.ProviderConfig, ignored map[string]bool) error {
	files, err := filepath.Glob(filepath.Join(outputDir, "*.go"))
	if err != nil {
		return fmt.Errorf("failed to list transformer files: %w", err)
	}

	for _, fullPath := range files {
		providerName := strings.TrimSuffix(filepath.Base(fullPath), ".go")
		relativePath := fmt.Sprintf("%s/%s.go", strings.TrimPrefix(outputDir, "./"), providerName)
		if providerName == "transformers" || ignored[relativePath] {
			continue
		}
		if _, exists := providers[providerName]; exists {
			continue
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", fullPath, err)
		}
		if !strings.HasPrefix(string(content), generatedHeader) {
			continue
		}

		if err := os.Remove(fullPath); err != nil {
			return fmt.Errorf("failed to remove stale %s: %w", fullPath, err)
		}
		fmt.Printf("Removed stale %s\n", fullPath)
	}

	return nil
}

// Providers generates individual provider files based on their configuration
// Only generates files not listed in .openapi-ignore and uses only the OpenAI template
func Providers(outputDir string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	if len(schema.Components.Schemas.Provider.XProviderConfigs) == 0 {
		return fmt.Errorf("no provider configurations found in OpenAPI spec")
	}

	ignoreFilePath := ".openapi-ignore"
	ignored, err := readIgnoreFile(ignoreFilePath)
	if err != nil {
		return fmt.Errorf("failed to read ignore file: %w", err)
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
	}

	openaiCompatibleTemplate := `// Code generated from OpenAPI schema. DO NOT EDIT.
package transformers

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

type ListModelsResponse{{.ProviderName}} struct {
	modelListing
}

func (l *ListModelsResponse{{.ProviderName}}) Transform() types.ListModelsResponse {
	provider := constants.{{.ProviderName}}ID
	return types.ListModelsResponse{
		Provider: &provider,
		Object:   l.Object,
		Data:     l.models(provider),
	}
}
`

	if err := removeStaleTransformers(outputDir, schema.Components.Schemas.Provider.XProviderConfigs, ignored); err != nil {
		return err
	}

	for providerName, config := range schema.Components.Schemas.Provider.XProviderConfigs {
		filename := fmt.Sprintf("%s.go", strings.ToLower(providerName))
		fullPath := fmt.Sprintf("%s/%s", outputDir, filename)

		relativePath := fmt.Sprintf("%s/%s", strings.TrimPrefix(outputDir, "./"), filename)

		if ignored[relativePath] {
			fmt.Printf("Skipping %s (found in .openapi-ignore)\n", relativePath)
			continue
		}

		tmpl, err := template.New(providerName).Funcs(funcMap).Parse(openaiCompatibleTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse template for %s: %w", providerName, err)
		}

		data := struct {
			ProviderName string
			Config       openapi.ProviderConfig
		}{
			ProviderName: funcMap["pascalCase"].(func(string) string)(providerName),
			Config:       config,
		}

		f, err := os.Create(fullPath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", fullPath, err)
		}

		if err := tmpl.Execute(f, data); err != nil {
			f.Close()
			return fmt.Errorf("failed to execute template for %s: %w", providerName, err)
		}

		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close %s: %w", fullPath, err)
		}

		cmd := exec.Command("gofmt", "-w", fullPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to format %s: %w", fullPath, err)
		}

		fmt.Printf("Generated %s\n", fullPath)
	}

	factoryTemplate := `// Code generated from OpenAPI schema. DO NOT EDIT.
package transformers

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// NewListModelsTransformer returns the list-models transformer for the given
// provider, falling back to the OpenAI-compatible transformer.
func NewListModelsTransformer(provider types.Provider) constants.ListModelsTransformer {
	switch provider {
	{{- range $name, $config := .Providers }}
	case constants.{{pascalCase $name}}ID:
		return &ListModelsResponse{{pascalCase $name}}{}
	{{- end }}
	default:
		return &ListModelsResponseOpenai{}
	}
}
`

	factoryPath := fmt.Sprintf("%s/transformers.go", outputDir)
	tmpl, err := template.New("transformers").Funcs(funcMap).Parse(factoryTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse transformers factory template: %w", err)
	}

	data := struct {
		Providers map[string]openapi.ProviderConfig
	}{
		Providers: schema.Components.Schemas.Provider.XProviderConfigs,
	}

	f, err := os.Create(factoryPath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", factoryPath, err)
	}

	if err := tmpl.Execute(f, data); err != nil {
		f.Close()
		return fmt.Errorf("failed to execute transformers factory template: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", factoryPath, err)
	}

	cmd := exec.Command("gofmt", "-w", factoryPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to format %s: %w", factoryPath, err)
	}

	fmt.Printf("Generated %s\n", factoryPath)
	return nil
}
