package openapi

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

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

// Structures for OpenAPI schema parsing
type SchemaProperty struct {
	Type       string                 `yaml:"type"`
	Properties map[string]SchemaField `yaml:"properties"`
	Items      *SchemaField           `yaml:"items"`
	Ref        string                 `yaml:"$ref"`
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
