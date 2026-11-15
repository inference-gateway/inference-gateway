package main

import (
	"flag"
	"fmt"
	"os"

	gen "github.com/inference-gateway/inference-gateway/internal/gen"
)

var (
	output string
	input  string
	_type  string
)

func init() {
	flag.StringVar(&output, "output", "", "Path to the output file")
	flag.StringVar(&input, "input", "", "Path to the input file (ModelsDev: a models.dev repository tarball)")
	flag.StringVar(&_type, "type", "", "The type of the file to generate (Env, MD, Config, Providers, ProviderRegistry, ProvidersClientConfig, ProvidersConstants, MCPWrap, or ModelsDev)")
}

func main() {
	flag.Parse()

	if output == "" || _type == "" {
		fmt.Println("Both -output and -type must be specified")
		os.Exit(1)
	}

	var err error
	switch _type {
	case "Env":
		fmt.Printf("Generating Dot Env to %s\n", output)
		err = gen.EnvExample(output, "openapi.yaml")
	case "MD":
		fmt.Printf("Generating Markdown to %s\n", output)
		err = gen.ConfigurationsMD(output, "openapi.yaml")
	case "ProvidersClientConfig":
		fmt.Printf("Generating providers client config to %s\n", output)
		err = gen.ProvidersClientConfig(output, "openapi.yaml")
	case "ProvidersConstants":
		fmt.Printf("Generating providers constants to %s\n", output)
		err = gen.Constants(output, "openapi.yaml")
	case "Config":
		fmt.Printf("Generating Go Config to %s\n", output)
		err = gen.Config(output, "openapi.yaml")
	case "Providers":
		fmt.Printf("Generating provider files to %s\n", output)
		err = gen.Providers(output, "openapi.yaml")
	case "ModelsDev":
		fmt.Printf("Generating models.dev metadata table to %s\n", output)
		if input == "" {
			fmt.Println("-input must point to a models.dev repository tarball")
			os.Exit(1)
		}
		err = gen.Models(output, input)
	case "MCPWrap":
		fmt.Printf("Wrapping MCP JSON schema as OpenAPI 3.1 to %s\n", output)
		err = gen.MCPWrap(output, "internal/mcp/mcp-schema.yaml")
	case "ProviderRegistry":
		fmt.Printf("Generating provider registry to %s\n", output)
		err = gen.ProviderRegistry(output, "openapi.yaml")
	default:
		fmt.Println("Invalid type specified")
		os.Exit(1)
	}
	if err == nil {
		return
	}
	fmt.Printf("Error generating %s: %v\n", _type, err)
	os.Exit(1)
}
