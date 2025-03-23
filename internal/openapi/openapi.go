package openapi

import (
	"fmt"
	"os"
	"reflect"

	"gopkg.in/yaml.v3"
)

func GetSchemas(schema *OpenAPISchema) map[string]SchemaProperty {
	result := make(map[string]SchemaProperty)

	v := reflect.ValueOf(schema.Components.Schemas)
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Skip Config and Providers as they have special structures
		if field.Name == "Config" || field.Name == "Provider" {
			continue
		}

		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" {
			continue
		}

		fieldValue := v.Field(i)
		if !fieldValue.IsValid() || fieldValue.IsZero() {
			continue
		}

		schemaProp := fieldValue.Interface().(SchemaProperty)
		if len(schemaProp.Properties) > 0 {
			populatePropertyNames(schemaProp.Properties)
		}

		result[field.Name] = schemaProp
	}

	return result
}

// populatePropertyNames sets the Name field for all properties in a map
func populatePropertyNames(properties map[string]Property) {
	for name, prop := range properties {
		newProp := prop
		newProp.Name = name

		if len(newProp.Properties) > 0 {
			populatePropertyNames(newProp.Properties)
		}

		if newProp.Items != nil {
			if newProp.Items.Name == "" {
				itemProp := *newProp.Items
				itemProp.Name = name + "Item"
				newProp.Items = &itemProp
			}

			if len(newProp.Items.Properties) > 0 {
				populatePropertyNames(newProp.Items.Properties)
			}
		}

		properties[name] = newProp
	}
}

// OpenAPI schema structures
type OpenAPISchema struct {
	Components struct {
		Schemas struct {
			Config   Config   `yaml:"Config"`
			Provider Provider `yaml:"Provider"`

			ProviderAuthType                      SchemaProperty `yaml:"ProviderAuthType"`
			MessageRole                           SchemaProperty `yaml:"MessageRole"`
			Message                               SchemaProperty `yaml:"Message"`
			Model                                 SchemaProperty `yaml:"Model"`
			ListModelsResponse                    SchemaProperty `yaml:"ListModelsResponse"`
			Endpoints                             SchemaProperty `yaml:"Endpoints"`
			Error                                 SchemaProperty `yaml:"Error"`
			FunctionObject                        SchemaProperty `yaml:"FunctionObject"`
			FunctionParameters                    SchemaProperty `yaml:"FunctionParameters"`
			FinishReason                          SchemaProperty `yaml:"FinishReason"`
			CompletionUsage                       SchemaProperty `yaml:"CompletionUsage"`
			ChatCompletionToolType                SchemaProperty `yaml:"ChatCompletionToolType"`
			ChatCompletionTool                    SchemaProperty `yaml:"ChatCompletionTool"`
			ChatCompletionChoice                  SchemaProperty `yaml:"ChatCompletionChoice"`
			ChatCompletionStreamChoice            SchemaProperty `yaml:"ChatCompletionStreamChoice"`
			CreateChatCompletionRequest           SchemaProperty `yaml:"CreateChatCompletionRequest"`
			CreateCompletionRequest               SchemaProperty `yaml:"CreateCompletionRequest"`
			CreateChatCompletionResponse          SchemaProperty `yaml:"CreateChatCompletionResponse"`
			ChatCompletionStreamOptions           SchemaProperty `yaml:"ChatCompletionStreamOptions"`
			CreateChatCompletionStreamResponse    SchemaProperty `yaml:"CreateChatCompletionStreamResponse"`
			ChatCompletionStreamResponseDelta     SchemaProperty `yaml:"ChatCompletionStreamResponseDelta"`
			ChatCompletionMessageToolCallChunk    SchemaProperty `yaml:"ChatCompletionMessageToolCallChunk"`
			ChatCompletionMessageToolCall         SchemaProperty `yaml:"ChatCompletionMessageToolCall"`
			ChatCompletionMessageToolCallFunction SchemaProperty `yaml:"ChatCompletionMessageToolCallFunction"`
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
	Name        string              `yaml:"name,omitempty"`
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
}

type SchemaField struct {
	Type       string                 `yaml:"type"`
	Properties map[string]SchemaField `yaml:"properties"`
	Items      *SchemaField           `yaml:"items"`
	Ref        string                 `yaml:"$ref"`
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
