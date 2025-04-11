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

	sdk "github.com/inference-gateway/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	homedir "k8s.io/client-go/util/homedir"
)

const systemPrompt = `You are a Kubernetes reliability engineer. Analyze this error log and:
1. Identify the root cause
2. Suggest solutions
3. Provide prevention tips
Keep response under 500 characters.

Error Log:
%s`

// Common error patterns to detect
var errorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)error`),
	regexp.MustCompile(`(?i)exception`),
	regexp.MustCompile(`(?i)fail`),
	regexp.MustCompile(`(?i)panic`),
	regexp.MustCompile(`(?i)timeout`),
	regexp.MustCompile(`(?i)denied`),
	regexp.MustCompile(`(?i)oom`),
	regexp.MustCompile(`(?i)crash`),
}

func main() {
	// Get Kubernetes config - works both in-cluster and locally
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

	// Initialize inference gateway client with proper options
	apiClient := sdk.NewClient(&sdk.ClientOptions{
		BaseURL: "http://inference-gateway.inference-gateway:8080/v1",
	})

	log.Println("Starting AI-powered Kubernetes log analyzer agent...")

	for {
		namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			log.Printf("Error listing namespaces: %v", err)
			continue
		}

		for _, ns := range namespaces.Items {
			pods, err := clientset.CoreV1().Pods(ns.Name).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				log.Printf("Error listing pods in namespace %s: %v", ns.Name, err)
				continue
			}

			for _, pod := range pods.Items {
				// Skip our own pod to avoid recursion
				if strings.Contains(pod.Name, "logs-analyzer") {
					continue
				}

				// Only analyze running/failed pods
				if pod.Status.Phase != v1.PodRunning && pod.Status.Phase != v1.PodFailed {
					continue
				}

				req := clientset.CoreV1().Pods(ns.Name).GetLogs(pod.Name, &v1.PodLogOptions{})
				logs, err := req.Stream(context.Background())
				if err != nil {
					log.Printf("Error getting logs from pod %s/%s: %v", ns.Name, pod.Name, err)
					continue
				}

				buf := new(strings.Builder)
				_, err = io.Copy(buf, logs)
				logs.Close()
				if err != nil {
					log.Printf("Error reading logs: %v", err)
					continue
				}

				// Check for error patterns in logs
				for _, line := range strings.Split(buf.String(), "\n") {
					for _, pattern := range errorPatterns {
						if pattern.MatchString(line) {
							ctx := context.Background()
							response, err := apiClient.GenerateContent(
								ctx,
								sdk.Groq,
								"llama-3.3-70b-versatile",
								[]sdk.Message{
									{
										Role:    sdk.System,
										Content: fmt.Sprintf(systemPrompt, line),
									},
									{
										Role:    sdk.User,
										Content: "Analyze this error",
									},
								},
							)
							if err != nil {
								log.Printf("Error analyzing log: %v", err)
								continue
							}

							log.Printf("Found error in %s/%s:\nError: %s\nAnalysis: %s",
								ns.Name, pod.Name, line, response.Choices[0].Message.Content)
							break
						}
					}
				}
			}
		}

		// Sleep before next scan
		time.Sleep(30 * time.Second)
	}
}
