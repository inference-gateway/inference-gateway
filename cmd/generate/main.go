package main

import (
	"flag"
	"fmt"
	"os"

	codegen "github.com/inference-gateway/inference-gateway/internal/codegen"
	dockergen "github.com/inference-gateway/inference-gateway/internal/dockergen"
	mdgen "github.com/inference-gateway/inference-gateway/internal/mdgen"
	pricinggen "github.com/inference-gateway/inference-gateway/internal/pricinggen"
)

var (
	output string
	input  string
	_type  string
)

func init() {
	flag.StringVar(&output, "output", "", "Path to the output file")
	flag.StringVar(&input, "input", "", "Path to the input file (CommunityPricing, CommunityContextWindows: a models.dev repository tarball)")
	flag.StringVar(&_type, "type", "", "The type of the file to generate (Env, MD, Config, Providers, ProviderRegistry, ProvidersClientConfig, ProvidersConstants, MCPWrap, CommunityPricing, or CommunityContextWindows)")
}

func main() {
	flag.Parse()

	if output == "" || _type == "" {
		fmt.Println("Both -output and -type must be specified")
		os.Exit(1)
	}

	switch _type {
	case "Env":
		fmt.Printf("Generating Dot Env to %s\n", output)
		err := dockergen.GenerateEnvExample(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating env example: %v\n", err)
			os.Exit(1)
		}
	case "MD":
		fmt.Printf("Generating Markdown to %s\n", output)
		err := mdgen.GenerateConfigurationsMD(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating MD: %v\n", err)
			os.Exit(1)
		}
	case "ProvidersClientConfig":
		fmt.Printf("Generating providers client config to %s\n", output)
		err := codegen.GenerateProvidersClientConfig(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating providers client config: %v\n", err)
			os.Exit(1)
		}
	case "ProvidersConstants":
		fmt.Printf("Generating providers constants to %s\n", output)
		err := codegen.GenerateConstants(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating providers constants: %v\n", err)
			os.Exit(1)
		}
	case "Config":
		fmt.Printf("Generating Go Config to %s\n", output)
		err := codegen.GenerateConfig(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating config: %v\n", err)
			os.Exit(1)
		}
	case "Providers":
		fmt.Printf("Generating provider files to %s\n", output)
		err := codegen.GenerateProviders(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating providers: %v\n", err)
			os.Exit(1)
		}
	case "CommunityPricing":
		fmt.Printf("Generating community pricing table to %s\n", output)
		if input == "" {
			fmt.Println("-input must point to a models.dev repository tarball")
			os.Exit(1)
		}
		err := pricinggen.Generate(output, input)
		if err != nil {
			fmt.Printf("Error generating community pricing table: %v\n", err)
			os.Exit(1)
		}
	case "CommunityContextWindows":
		fmt.Printf("Generating community context-window table to %s\n", output)
		if input == "" {
			fmt.Println("-input must point to a models.dev repository tarball")
			os.Exit(1)
		}
		err := pricinggen.GenerateContextWindows(output, input)
		if err != nil {
			fmt.Printf("Error generating community context-window table: %v\n", err)
			os.Exit(1)
		}
	case "MCPWrap":
		fmt.Printf("Wrapping MCP JSON schema as OpenAPI 3.1 to %s\n", output)
		err := codegen.GenerateMCPWrap(output, "internal/mcp/mcp-schema.yaml")
		if err != nil {
			fmt.Printf("Error wrapping MCP schema: %v\n", err)
			os.Exit(1)
		}
	case "ProviderRegistry":
		fmt.Printf("Generating provider registry to %s\n", output)
		err := codegen.GenerateProviderRegistry(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating provider registry: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Println("Invalid type specified")
		os.Exit(1)
	}
}
