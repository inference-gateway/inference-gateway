package main

import (
	"flag"
	"fmt"
	"sort"

	"os"
	"strings"

	"github.com/inference-gateway/inference-gateway/internal/codegen"
	"github.com/inference-gateway/inference-gateway/internal/mdgen"
)

var (
	output string
	_type  string
)

func init() {
	flag.StringVar(&output, "output", "", "Path to the output file")
	flag.StringVar(&_type, "type", "", "The type of the file to generate (Env, ConfigMap, Secret, or MD)")
}

func main() {
	flag.Parse()

	if output == "" || _type == "" {
		fmt.Println("Both -output and -type must be specified")
		os.Exit(1)
	}

	switch _type {
	case "Env":
		// dockergen.GenerateEnvExample(output)
	case "ConfigMap":
		// kubegen.GenerateConfigMap(output)
	case "Secret":
		// kubegen.GenerateSecret(output)
	case "MD":
		err := mdgen.GenerateConfigurationsMD(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating MD: %v\n", err)
			os.Exit(1)
		}
	case "Providers":
		if err := codegen.GenerateProviders(output, "openapi.yaml"); err != nil {
			fmt.Printf("Error generating providers: %v\n", err)
			os.Exit(1)
		}
	case "Config":
		err := codegen.GenerateConfig(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating config: %v\n", err)
			os.Exit(1)
		}
		err = codegen.GenerateProvidersRegistry("providers/registry.go", "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating providers registry: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Println("Invalid type specified")
		os.Exit(1)
	}
}

// func generateFromConfig(cfg *config.Config) map[string]FieldInfo {
// 	fields := make(map[string]FieldInfo)
// 	t := reflect.TypeOf(*cfg)

// 	// Process base struct fields
// 	for i := 0; i < t.NumField(); i++ {
// 		field := t.Field(i)
// 		tag := field.Tag.Get("env")
// 		desc := field.Tag.Get("description")
// 		if tag == "" {
// 			continue
// 		}

// 		// Parse env tag
// 		envName, defaultValue, isSecret := parseEnvTag(tag)
// 		fields[field.Name] = FieldInfo{
// 			EnvName:      envName,
// 			DefaultValue: defaultValue,
// 			Description:  desc,
// 			IsSecret:     isSecret,
// 		}

// 		// Handle nested structs (OIDC, Server, Provider configs)
// 		if field.Type.Kind() == reflect.Ptr {
// 			nestedFields := processNestedStruct(field)
// 			for k, v := range nestedFields {
// 				fields[k] = v
// 			}
// 		}
// 	}
// 	return fields
// }

// func processNestedStruct(field reflect.StructField) map[string]FieldInfo {
// 	fields := make(map[string]FieldInfo)

// 	structType := field.Type.Elem()
// 	prefix := ""

// 	// Extract clean prefix from env tag
// 	if tag := field.Tag.Get("env"); tag != "" {
// 		parts := strings.Split(tag, ",")
// 		for _, part := range parts {
// 			part = strings.TrimSpace(part)
// 			if strings.HasPrefix(part, "prefix=") {
// 				prefix = strings.Trim(strings.TrimPrefix(part, "prefix="), "\"")
// 				break
// 			}
// 		}
// 	}

// 	// Process nested struct fields
// 	for i := 0; i < structType.NumField(); i++ {
// 		nestedField := structType.Field(i)
// 		tag := nestedField.Tag.Get("env")
// 		desc := nestedField.Tag.Get("description")

// 		if tag == "" {
// 			continue
// 		}

// 		// Parse env tag without prefix=
// 		envName, defaultValue, isSecret := parseEnvTag(tag)

// 		// Add clean prefix to env name
// 		if prefix != "" {
// 			envName = prefix + envName
// 		}

// 		fields[nestedField.Name] = FieldInfo{
// 			EnvName:      envName,
// 			DefaultValue: defaultValue,
// 			Description:  desc,
// 			IsSecret:     isSecret,
// 		}

// 		// Handle nested struct fields recursively
// 		if nestedField.Type.Kind() == reflect.Ptr {
// 			nestedFields := processNestedStruct(nestedField)
// 			for k, v := range nestedFields {
// 				fields[k] = v
// 			}
// 		}
// 	}

// 	return fields
// }

// func generateEnvExample(filePath string) error {
// 	cfg := &config.Config{}
// 	fields := generateFromConfig(cfg)

// 	var sb strings.Builder
// 	for _, field := range fields {
// 		if field.Description != "" {
// 			sb.WriteString(fmt.Sprintf("# %s\n", field.Description))
// 		}
// 		sb.WriteString(fmt.Sprintf("%s=%s\n", field.EnvName, field.DefaultValue))
// 	}

// 	return os.WriteFile(filePath, []byte(sb.String()), 0644)
// }

// func generateConfigMap(filePath string) error {
// 	cfg := &config.Config{}
// 	fields := generateFromConfig(cfg)

// 	// Group fields by category
// 	general := make(map[string]FieldInfo)
// 	server := make(map[string]FieldInfo)
// 	api := make(map[string]FieldInfo)

// 	// Provider prefixes to ensure we catch all
// 	providers := []string{
// 		"OLLAMA_",
// 		"GROQ_",
// 		"OPENAI_",
// 		"GOOGLE_",
// 		"CLOUDFLARE_",
// 		"COHERE_",
// 		"ANTHROPIC_",
// 	}

// 	for _, field := range fields {
// 		if field.IsSecret {
// 			continue
// 		}

// 		// Check if field belongs to a provider
// 		isProviderField := false
// 		for _, prefix := range providers {
// 			if strings.HasPrefix(field.EnvName, prefix) {
// 				if strings.HasSuffix(field.EnvName, "_API_URL") {
// 					api[field.EnvName] = field
// 					isProviderField = true
// 					break
// 				}
// 			}
// 		}

// 		// If not a provider field, categorize normally
// 		if !isProviderField {
// 			switch {
// 			case strings.HasPrefix(field.EnvName, "SERVER_"):
// 				server[field.EnvName] = field
// 			case strings.HasPrefix(field.EnvName, "OIDC_"):
// 				if !strings.Contains(field.EnvName, "SECRET") {
// 					general[field.EnvName] = field
// 				}
// 			case !strings.Contains(field.EnvName, "API_KEY") &&
// 				!strings.Contains(field.EnvName, "TOKEN"):
// 				general[field.EnvName] = field
// 			}
// 		}
// 	}

// 	var sb strings.Builder
// 	sb.WriteString("---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n")
// 	sb.WriteString("  name: inference-gateway\n")
// 	sb.WriteString("  namespace: inference-gateway\n")
// 	sb.WriteString("  labels:\n    app: inference-gateway\n")
// 	sb.WriteString("data:\n")

// 	// Write general settings
// 	if len(general) > 0 {
// 		sb.WriteString("  # General settings\n")
// 		writeFields(&sb, general)
// 	}

// 	// Write server settings
// 	if len(server) > 0 {
// 		sb.WriteString("  # Server settings\n")
// 		writeFields(&sb, server)
// 	}

// 	// Write API URLs
// 	if len(api) > 0 {
// 		sb.WriteString("  # API URLs and keys\n")
// 		writeFields(&sb, api)
// 	}

// 	return os.WriteFile(filePath, []byte(sb.String()), 0644)
// }

func writeFields(sb *strings.Builder, fields map[string]FieldInfo) {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	for _, key := range keys {
		field := fields[key]
		if field.DefaultValue != "" {
			sb.WriteString(fmt.Sprintf("  %s: \"%s\"\n", field.EnvName, field.DefaultValue))
		} else {
			sb.WriteString(fmt.Sprintf("  %s: \"\"\n", field.EnvName))
		}
	}
}

// func generateSecret(filePath string) error {
// 	cfg := &config.Config{}
// 	fields := generateFromConfig(cfg)

// 	var sb strings.Builder
// 	sb.WriteString("apiVersion: v1\nkind: Secret\nmetadata:\n  name: inference-gateway\ntype: Opaque\nstringData:\n")

// 	for _, field := range fields {
// 		if field.IsSecret {
// 			if field.Description != "" {
// 				sb.WriteString(fmt.Sprintf("  # %s\n", field.Description))
// 			}
// 			sb.WriteString(fmt.Sprintf("  %s: \"\"\n", field.EnvName))
// 		}
// 	}

// 	return os.WriteFile(filePath, []byte(sb.String()), 0644)
// }

type FieldInfo struct {
	EnvName      string
	DefaultValue string
	Description  string
	IsSecret     bool
}

func parseEnvTag(tag string) (envName, defaultValue string, isSecret bool) {
	parts := strings.Split(tag, ",")
	envName = strings.TrimSpace(parts[0])

	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "default=") {
			defaultValue = strings.Trim(strings.TrimPrefix(part, "default="), "\"")
		}
		if strings.Contains(part, "type:\"secret\"") {
			isSecret = true
		}
	}

	return
}
