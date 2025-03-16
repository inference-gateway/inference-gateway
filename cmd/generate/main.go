package main

import (
	"flag"
	"fmt"

	"os"

	"github.com/inference-gateway/inference-gateway/internal/codegen"
	"github.com/inference-gateway/inference-gateway/internal/dockergen"
	"github.com/inference-gateway/inference-gateway/internal/kubegen"
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
		fmt.Printf("Generating Dot Env to %s\n", output)
		err := dockergen.GenerateEnvExample(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating env example: %v\n", err)
			os.Exit(1)
		}
	case "ConfigMap":
		fmt.Printf("Generating Kubernetes ConfigMap to %s\n", output)
		err := kubegen.GenerateConfigMap(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating config map: %v\n", err)
			os.Exit(1)
		}
	case "Secret":
		fmt.Printf("Generating Kubernetes Secret to %s\n", output)
		err := kubegen.GenerateSecret(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating secret: %v\n", err)
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
	case "ProvidersCommonTypes":
		fmt.Printf("Generating providers common types to %s\n", output)
		err := codegen.GenerateCommonTypes("providers/common_types.go", "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating providers common types: %v\n", err)
			os.Exit(1)
		}
	case "Config":
		fmt.Printf("Generating Go Config to %s\n", output)
		err := codegen.GenerateConfig(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating config: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Println("Invalid type specified")
		os.Exit(1)
	}
}
