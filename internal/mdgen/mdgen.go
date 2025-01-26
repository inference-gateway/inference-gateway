package mdgen

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/inference-gateway/inference-gateway/internal/openapi"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func GenerateConfigurationsMD(filePath string, oas string) error {
	// Read OpenAPI spec
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	configSchema := schema.Components.Schemas.Config.XConfig
	providers := schema.Components.Schemas.Providers.XProviderConfigs

	var sb strings.Builder
	sb.WriteString("# Inference Gateway Configuration\n\n")

	// Helper function to write section
	writeSection := func(title string, fields map[string]openapi.ConfigField) {
		sb.WriteString(fmt.Sprintf("## %s\n\n", title))
		sb.WriteString("| Environment Variable | Default Value | Description |\n")
		sb.WriteString("|---------------------|---------------|-------------|\n")

		// Sort keys for consistent output
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			field := fields[key]
			defaultVal := "`" + field.Default + "`"
			if field.Default == "" {
				defaultVal = "`\"\"`"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
				field.Env,
				defaultVal,
				field.Description,
			))
		}
		sb.WriteString("\n")
	}

	// Write General Settings
	generalFields := map[string]openapi.ConfigField{
		"application_name": configSchema.General.ApplicationName,
		"environment":      configSchema.General.Environment,
		"enable_telemetry": configSchema.General.EnableTelemetry,
		"enable_auth":      configSchema.General.EnableAuth,
	}
	writeSection("General Settings", generalFields)

	// Write OIDC Settings
	oidcFields := map[string]openapi.ConfigField{
		"issuer_url":    configSchema.OIDC.IssuerURL,
		"client_id":     configSchema.OIDC.ClientID,
		"client_secret": configSchema.OIDC.ClientSecret,
	}
	writeSection("OIDC Settings", oidcFields)

	// Write Server Settings
	serverFields := map[string]openapi.ConfigField{
		"host":          configSchema.Server.Host,
		"port":          configSchema.Server.Port,
		"read_timeout":  configSchema.Server.ReadTimeout,
		"write_timeout": configSchema.Server.WriteTimeout,
		"idle_timeout":  configSchema.Server.IdleTimeout,
		"tls_cert_path": configSchema.Server.TLSCertPath,
		"tls_key_path":  configSchema.Server.TLSKeyPath,
	}
	writeSection("Server Settings", serverFields)

	// Write Provider Settings
	sb.WriteString("## API URLs and keys\n\n")
	sb.WriteString("| Environment Variable | Default Value | Description |\n")
	sb.WriteString("|---------------------|---------------|-------------|\n")

	// Sort provider names for consistent output
	providerNames := make([]string, 0, len(providers))
	for name := range providers {
		providerNames = append(providerNames, name)
	}
	sort.Strings(providerNames)

	// Write provider configurations
	caser := cases.Title(language.English)
	for _, name := range providerNames {
		config := providers[name]
		urlVar := strings.ToUpper(name) + "_API_URL"
		tokenVar := strings.ToUpper(name) + "_API_KEY"

		// Write URL
		sb.WriteString(fmt.Sprintf("| %s | `%s` | The URL for %s API |\n",
			urlVar,
			config.URL,
			caser.String(name),
		))

		// Write Token if provider requires authentication
		if config.AuthType != "none" {
			sb.WriteString(fmt.Sprintf("| %s | `\"\"` | The Access token for %s API |\n",
				tokenVar,
				caser.String(name),
			))
		}
	}
	sb.WriteString("\n")

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}
