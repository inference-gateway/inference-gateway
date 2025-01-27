package openapi

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func GetSchemas(schema *OpenAPISchema) map[string]SchemaProperty {
	return map[string]SchemaProperty{
		"AuthType":           schema.Components.Schemas.AuthType,
		"Message":            schema.Components.Schemas.Message,
		"Model":              schema.Components.Schemas.Model,
		"ListModelsResponse": schema.Components.Schemas.ListResponse,
		"GenerateRequest":    schema.Components.Schemas.GenerateRequest,
		"GenerateResponse":   schema.Components.Schemas.GenerateResponse,
		"ResponseTokens":     schema.Components.Schemas.ResponseToken,
	}
}

// OpenAPI schema structures
type OpenAPISchema struct {
	Components struct {
		Schemas struct {
			Config struct {
				XConfig ConfigSchema `yaml:"x-config"`
			} `yaml:"Config"`
			Providers struct {
				XProviderConfigs map[string]ProviderConfig `yaml:"x-provider-configs"`
			} `yaml:"Providers"`
			AuthType         SchemaProperty `yaml:"AuthType"`
			Message          SchemaProperty `yaml:"Message"`
			Model            SchemaProperty `yaml:"Model"`
			ListResponse     SchemaProperty `yaml:"ListModelsResponse"`
			GenerateRequest  SchemaProperty `yaml:"GenerateRequest"`
			GenerateResponse SchemaProperty `yaml:"GenerateResponse"`
			ResponseToken    SchemaProperty `yaml:"ResponseTokens"`
		}
	}
}

type ConfigSchema struct {
	Sections []map[string]Section `yaml:"sections"`
}

type Section struct {
	Title    string                   `yaml:"title"`
	Settings []map[string]ConfigField `yaml:"settings"`
}

type ConfigField struct {
	Env         string `yaml:"env"`
	Default     string `yaml:"default,omitempty"`
	Description string `yaml:"description"`
	Secret      bool   `yaml:"secret,omitempty"`
}

// ExtraHeader can be either string or []string
type ExtraHeader struct {
	Values []string
}

// UnmarshalYAML implements custom unmarshaling for ExtraHeader
func (h *ExtraHeader) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		h.Values = []string{value.Value}
	case yaml.SequenceNode:
		var values []string
		if err := value.Decode(&values); err != nil {
			return err
		}
		h.Values = values
	default:
		return fmt.Errorf("unexpected header value type")
	}
	return nil
}

type ProviderEndpoints struct {
	List     string `yaml:"list"`
	Generate string `yaml:"generate"`
}

type Transform struct {
	Target  string       `yaml:"target"`
	Mapping TransformMap `yaml:"mapping"`
}

type TransformMap struct {
	Provider string   `yaml:"provider"`
	Models   ModelMap `yaml:"models,omitempty"`
}

type ModelMap struct {
	Source    string         `yaml:"source,omitempty"`
	Transform []TransformRef `yaml:"transform"`
}

type TransformRef struct {
	Source   string `yaml:"source,omitempty"`
	Target   string `yaml:"target"`
	Constant string `yaml:"constant,omitempty"`
}

type Property struct {
	Type        string              `yaml:"type"`
	Format      string              `yaml:"format,omitempty"`
	Description string              `yaml:"description,omitempty"`
	Ref         string              `yaml:"$ref,omitempty"`
	Enum        []string            `yaml:"enum,omitempty"`
	Properties  map[string]Property `yaml:"properties,omitempty"`
	Items       *Property           `yaml:"items,omitempty"`
}

// Structures for OpenAPI schema parsing
type SchemaProperty struct {
	Type        string              `yaml:"type"`
	Description string              `yaml:"description"`
	Properties  map[string]Property `yaml:"properties"`
	Required    []string            `yaml:"required,omitempty"`
	Items       *Property           `yaml:"items,omitempty"`
	Enum        []string            `yaml:"enum,omitempty"`
	XTransform  *Transform          `yaml:"x-transform,omitempty"`
}

type SchemaField struct {
	Type       string                 `yaml:"type"`
	Properties map[string]SchemaField `yaml:"properties"`
	Items      *SchemaField           `yaml:"items"`
	Ref        string                 `yaml:"$ref"`
}

type EndpointSchema struct {
	Endpoint string `yaml:"endpoint"`
	Method   string `yaml:"method"`
	Schema   struct {
		Request  SchemaProperty `yaml:"request"`
		Response SchemaProperty `yaml:"response"`
	} `yaml:"schema"`
}

type ProviderConfig struct {
	ID           string                    `yaml:"id"`
	URL          string                    `yaml:"url"`
	AuthType     string                    `yaml:"auth_type"`
	ExtraHeaders map[string]ExtraHeader    `yaml:"extra_headers"`
	Endpoints    map[string]EndpointSchema `yaml:"endpoints"`
}

func Read(openapi string) (*OpenAPISchema, error) {
	data, err := os.ReadFile(openapi)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	var schema OpenAPISchema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	return &schema, nil
}
