package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"time"

	filepath "path/filepath"

	sdk "github.com/edenreich/inference-gateway-go-sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	homedir "k8s.io/client-go/util/homedir"
	yaml "sigs.k8s.io/yaml"
)

const prompt = `
# System Instructions
You are an AI assistant specialized in site reliability engineering.
Your task is to analyze the following error log and provide a detailed summary of the issue along with potential solutions.

# Error Log
%s

# Instructions
1. Identify the root cause of the error.
2. Suggest potential solutions to resolve the issue.
3. Provide any additional recommendations to prevent similar issues in the future.

# Output
- The length of the answer should be max 1000 characters.
- Provide a concise summary of the issue.
- List potential solutions with examples.
- Mention the Pod, Namespace, and any other relevant information.
`

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("Error creating Kubernetes client config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	provider := "groq"                 // Using Groq API for analysis, but you can also use local Ollama provider if needed
	model := "llama-3.3-70b-versatile" // Using the Llama model for analysis
	apiClient := sdk.NewClient("http://inference-gateway.inference-gateway:8080")

	for {
		var errorLogs []string
		namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			log.Fatalf("Error listing namespaces: %v", err)
		}

		for _, ns := range namespaces.Items {
			pods, err := clientset.CoreV1().Pods(ns.Name).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				log.Fatalf("Error listing pods in namespace %s: %v", ns.Name, err)
			}

			for _, pod := range pods.Items {
				// Ignore logs from the logs-analyzer pod itself, otherwise there is an infinite logs accumulation loop, because the response from the LLM might contain the word error
				if pod.Namespace == "logs-analyzer" && strings.Contains(pod.Name, "logs-analyzer") {
					continue
				}

				// Only analyze running pods and user space errors
				if pod.Status.Phase != v1.PodRunning && pod.Status.Phase != v1.PodFailed && pod.Status.Phase != v1.PodSucceeded {
					continue
				}

				req := clientset.CoreV1().Pods(ns.Name).GetLogs(pod.Name, &v1.PodLogOptions{})
				logs, err := req.Stream(context.Background())
				if err != nil {
					log.Printf("Error getting logs from pod %s in namespace %s: %v", pod.Name, ns.Name, err)
					continue
				}

				buf := new(strings.Builder)
				_, err = io.Copy(buf, logs)
				if err != nil {
					log.Fatalf("Error reading logs: %v", err)
				}
				// Check for any line that contains the word "error" in the logs case-insensitively
				// Use regular expression to match lines containing the word "error" (case-insensitively)
				re := regexp.MustCompile(`(?i)error`)
				for _, line := range strings.Split(buf.String(), "\n") {
					if re.MatchString(line) {
						podSpec, err := yaml.Marshal(pod.Spec)
						if err != nil {
							log.Fatalf("Error marshalling pod spec: %v", err)
						}
						errorLog := fmt.Sprintf("Error: %s\nNamespace: %s\nPod: %s\nSpec: %s", line, ns.Name, pod.Name, string(podSpec))
						errorLogs = append(errorLogs, errorLog)
					}
				}
				logs.Close()
			}
		}

		// Add some safety cost and compute guards - limit the number of error logs to analyze, take the latest ones.
		maxErrorLogs := 10
		if len(errorLogs) > maxErrorLogs {
			errorLogs = errorLogs[len(errorLogs)-maxErrorLogs:]
		}

		fmt.Printf("Analyzing %d error logs...\n", len(errorLogs))
		for _, errorLog := range errorLogs {
			fmt.Println(errorLog)
		}

		for _, errorLog := range errorLogs {
			// Add some safety cost and compute guards - truncate the error log to max 100 characters.
			if len(errorLog) > 100 {
				errorLog = errorLog[:100]
			}

			response, err := apiClient.GenerateContent(provider, model, fmt.Sprintf(prompt, errorLog))
			if err != nil {
				log.Printf("Error analyzing log: %v", err)
				continue
			}
			fmt.Printf("Analysis result: %s\n", response.Response.Content)
		}

		// Sleep for 1min before analyzing logs again
		time.Sleep(1 * time.Minute)
	}
}
