package main

import (
	"flag"
	"fmt"

	"os"

	"github.com/inference-gateway/inference-gateway/internal/codegen"
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
		// dockergen.GenerateEnvExample(output)
	case "ConfigMap":
		fmt.Printf("Generating ConfigMap to %s\n", output)
		err := kubegen.GenerateConfigMap(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating config map: %v\n", err)
			os.Exit(1)
		}
	case "Secret":
		fmt.Printf("Generating Secret to %s\n", output)
		err := kubegen.GenerateSecret(output, "openapi.yaml")
		if err != nil {
			fmt.Printf("Error generating secret: %v\n", err)
			os.Exit(1)
		}
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
