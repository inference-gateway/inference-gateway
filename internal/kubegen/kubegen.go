package kubegen

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/inference-gateway/inference-gateway/internal/openapi"
	"gopkg.in/yaml.v3"
)

// SecretData represents the structure for Kubernetes Secret
type SecretData struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string            `yaml:"name"`
		Namespace string            `yaml:"namespace"`
		Labels    map[string]string `yaml:"labels"`
	} `yaml:"metadata"`
	Type       string            `yaml:"type"`
	StringData map[string]string `yaml:"stringData"`
}

func GenerateSecret(filePath string, oas string) error {
	// Read OpenAPI spec
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	// Initialize secret structure
	secret := SecretData{
		APIVersion: "v1",
		Kind:       "Secret",
		Type:       "Opaque",
	}
	secret.Metadata.Name = "inference-gateway"
	secret.Metadata.Namespace = "inference-gateway"
	secret.Metadata.Labels = map[string]string{
		"app": "inference-gateway",
	}
	secret.StringData = make(map[string]string)

	// Get config schema
	configSchema := schema.Components.Schemas.Config.XConfig

	// Process OIDC secrets
	if configSchema.OIDC.IssuerURL.Secret {
		secret.StringData[configSchema.OIDC.IssuerURL.Env] = ""
	}
	if configSchema.OIDC.ClientID.Secret {
		secret.StringData[configSchema.OIDC.ClientID.Env] = ""
	}
	if configSchema.OIDC.ClientSecret.Secret {
		secret.StringData[configSchema.OIDC.ClientSecret.Env] = ""
	}

	// Process provider secrets
	providers := schema.Components.Schemas.Providers.XProviderConfigs
	for name := range providers {
		tokenEnv := strings.ToUpper(name) + "_API_KEY"
		secret.StringData[tokenEnv] = ""
	}

	// Sort stringData keys for consistent output
	sortedKeys := make([]string, 0, len(secret.StringData))
	for k := range secret.StringData {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// Create sorted stringData
	sortedData := make(map[string]string)
	for _, k := range sortedKeys {
		sortedData[k] = secret.StringData[k]
	}
	secret.StringData = sortedData

	// Create file with proper indentation
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	// Write document separator
	if _, err := f.WriteString("---\n"); err != nil {
		return fmt.Errorf("failed to write separator: %w", err)
	}

	// Create encoder with 2-space indent
	encoder := yaml.NewEncoder(f)
	encoder.SetIndent(2)
	defer encoder.Close()

	// Encode secret struct
	if err := encoder.Encode(secret); err != nil {
		return fmt.Errorf("failed to encode secret: %w", err)
	}

	return nil
}
