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
			Config   Config   `yaml:"Config"`
			Provider Provider `yaml:"Provider"`
		}
	}
}

type Config struct {
	XConfig ConfigSchema `yaml:"x-config"`
}

type Provider struct {
	XProviderConfigs map[string]ProviderConfig `yaml:"x-provider-configs"`
}

type ConfigSchema struct {
	Sections []map[string]Section `yaml:"sections"`
}

type Section struct {
	Title    string    `yaml:"title"`
	Settings []Setting `yaml:"settings"`
}

type Setting struct {
	Env         string `yaml:"env"`
	Type        string `yaml:"type"`
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

type EndpointSchema struct {
	Name     string `yaml:"name"`
	Method   string `yaml:"method"`
	Endpoint string `yaml:"endpoint"`
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
