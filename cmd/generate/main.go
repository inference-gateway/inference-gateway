package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"

	config "github.com/edenreich/inference-gateway/config"
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

	comments := parseStructComments("config.go", "Config")

	switch _type {
	case "Env":
		generateEnvExample(output, comments)
	case "ConfigMap":
		generateConfigMap(output, comments)
	case "Secret":
		generateSecret(output, comments)
	case "MD":
		generateMD(output, comments)
	default:
		fmt.Println("Invalid type specified")
		os.Exit(1)
	}
}

func parseStructComments(filename, structName string) map[string]string {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		fmt.Printf("Error parsing file: %v\n", err)
		os.Exit(1)
	}

	comments := make(map[string]string)
	ast.Inspect(node, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != structName {
			return true
		}

		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}

		for _, field := range st.Fields.List {
			if field.Doc != nil {
				for _, comment := range field.Doc.List {
					comments[field.Names[0].Name] = strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
				}
			}
		}
		return false
	})

	return comments
}

func generateEnvExample(filePath string, comments map[string]string) {
	var cfg config.Config
	v := reflect.ValueOf(cfg)
	t := v.Type()

	var sb strings.Builder
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		envTag := field.Tag.Get("env")
		if envTag == "" {
			continue
		}
		envParts := strings.Split(envTag, ",")
		envName := envParts[0]
		defaultValue := ""
		for _, part := range envParts {
			if strings.HasPrefix(part, "default=") {
				defaultValue = strings.TrimPrefix(part, "default=")
				break
			}
		}
		if comment, ok := comments[field.Name]; ok {
			sb.WriteString(fmt.Sprintf("# %s\n", comment))
		}
		sb.WriteString(fmt.Sprintf("%s=%s\n", envName, defaultValue))
	}

	err := os.WriteFile(filePath, []byte(sb.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing %s: %v\n", filePath, err)
	}
}

func generateConfigMap(filePath string, comments map[string]string) {
	var cfg config.Config
	v := reflect.ValueOf(cfg)
	t := v.Type()

	var sb strings.Builder
	sb.WriteString("---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: inference-gateway\n  namespace: inference-gateway\n  labels:\n    app: inference-gateway\ndata:\n")

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		envTag := field.Tag.Get("env")
		typeTag := field.Tag.Get("type")
		if typeTag == "secret" {
			continue
		}

		if envTag == "" {
			continue
		}
		envParts := strings.Split(envTag, ",")
		envName := envParts[0]

		defaultValue := ""
		for _, part := range envParts {
			part = strings.Trim(part, " ")
			if strings.HasPrefix(part, "default=") {
				if envName == "OLLAMA_API_URL" {
					defaultValue = "http://ollama.ollama:8080"
					break
				}

				defaultValue = strings.TrimPrefix(part, "default=")
				break
			}
		}
		if comment, ok := comments[field.Name]; ok {
			sb.WriteString(fmt.Sprintf("  # %s\n", comment))
		}
		sb.WriteString(fmt.Sprintf("  %s: \"%s\"\n", envName, defaultValue))
	}

	err := os.WriteFile(filePath, []byte(sb.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing %s: %v\n", filePath, err)
	}
}

func generateSecret(filePath string, comments map[string]string) {
	var cfg config.Config
	v := reflect.ValueOf(cfg)
	t := v.Type()

	var sb strings.Builder
	sb.WriteString("---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: inference-gateway\n  namespace: inference-gateway\n  labels:\n    app: inference-gateway\ntype: Opaque\nstringData:\n")

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		envTag := field.Tag.Get("env")
		typeTag := field.Tag.Get("type")
		if typeTag != "secret" {
			continue
		}

		if envTag == "" {
			continue
		}
		envParts := strings.Split(envTag, ",")
		envName := envParts[0]

		if comment, ok := comments[field.Name]; ok {
			sb.WriteString(fmt.Sprintf("  # %s\n", comment))
		}
		sb.WriteString(fmt.Sprintf("  %s: \"\"\n", envName))
	}

	err := os.WriteFile(filePath, []byte(sb.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing %s: %v\n", filePath, err)
	}
}

func generateMD(filePath string, comments map[string]string) {
	var cfg config.Config
	v := reflect.ValueOf(cfg)
	t := v.Type()

	var sb strings.Builder
	sb.WriteString("# Inference Gateway Configuration\n")

	currentGroup := ""
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		envTag := field.Tag.Get("env")
		if envTag == "" {
			continue
		}
		envParts := strings.Split(envTag, ",")
		envName := envParts[0]
		description := field.Tag.Get("description")
		defaultValue := ""
		for _, part := range envParts {
			part = strings.Trim(part, " ")
			if strings.HasPrefix(part, "default=") {
				defaultValue = strings.TrimPrefix(part, "default=")
				break
			}
		}

		group := comments[field.Name]
		if group != currentGroup {
			if group != "" {
				sb.WriteString("\n")
				sb.WriteString(fmt.Sprintf("## %s\n\n", group))
				sb.WriteString("| Key | Default Value | Description |\n")
				sb.WriteString("| --- | ------------- | ----------- |\n")
				currentGroup = group
			}
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", envName, defaultValue, description))
	}

	err := os.WriteFile(filePath, []byte(sb.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing %s: %v\n", filePath, err)
	}
}
