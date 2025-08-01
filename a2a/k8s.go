package a2a

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	dynamic "k8s.io/client-go/dynamic"
	kubernetes "k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
)

// KubernetesServiceDiscovery handles Kubernetes-based service discovery for Agent resources
type KubernetesServiceDiscovery struct {
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
	namespace     string
	logger        logger.Logger
	config        *config.A2AConfig
}

// IsKubernetesEnvironment detects if the application is running in a Kubernetes environment
func IsKubernetesEnvironment() bool {
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		return true
	}

	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		if _, err := os.Stat(kubeconfig); err == nil {
			return true
		}
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		defaultKubeconfig := filepath.Join(homeDir, ".kube", "config")
		if _, err := os.Stat(defaultKubeconfig); err == nil {
			return true
		}
	}

	return false
}

// NewKubernetesServiceDiscovery creates a new Kubernetes service discovery instance
func NewKubernetesServiceDiscovery(cfg *config.A2AConfig, logger logger.Logger) (*KubernetesServiceDiscovery, error) {
	if !IsKubernetesEnvironment() {
		return nil, fmt.Errorf("not running in Kubernetes environment")
	}

	client, err := createKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	dynamicClient, err := createDynamicClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	namespace := cfg.ServiceDiscoveryNamespace
	if namespace == "" {
		if namespaceBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			namespace = strings.TrimSpace(string(namespaceBytes))
		}

		if namespace == "" {
			namespace = "default"
		}
	}

	return &KubernetesServiceDiscovery{
		client:        client,
		dynamicClient: dynamicClient,
		namespace:     namespace,
		logger:        logger,
		config:        cfg,
	}, nil
}

// createKubernetesClient creates a Kubernetes client using in-cluster config or kubeconfig
func createKubernetesClient() (kubernetes.Interface, error) {
	config, err := getKubernetesConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return client, nil
}

// createDynamicClient creates a dynamic client using in-cluster config or kubeconfig
func createDynamicClient() (dynamic.Interface, error) {
	config, err := getKubernetesConfig()
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return client, nil
}

// getKubernetesConfig gets the Kubernetes configuration
func getKubernetesConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				kubeconfig = filepath.Join(homeDir, ".kube", "config")
			}
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kubernetes config: %w", err)
		}
	}

	return config, nil
}

// DiscoverA2AServices discovers Agent services in the Kubernetes cluster using Agent CRDs
func (k *KubernetesServiceDiscovery) DiscoverA2AServices(ctx context.Context) ([]string, error) {
	agentGVR := schema.GroupVersionResource{
		Group:    "core.inference-gateway.com",
		Version:  "v1alpha1",
		Resource: "agents",
	}

	agentList, err := k.dynamicClient.Resource(agentGVR).Namespace(k.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Agent resources: %w", err)
	}

	var agentURLs []string
	for _, agent := range agentList.Items {
		agentName := agent.GetName()

		service, err := k.client.CoreV1().Services(k.namespace).Get(ctx, agentName, metav1.GetOptions{})
		if err != nil {
			k.logger.Warn("failed to get service for Agent resource",
				"agent", agentName,
				"namespace", k.namespace,
				"error", err,
				"component", "k8s_service_discovery")
			continue
		}

		agentURL := k.buildServiceURL(service)
		if agentURL != "" {
			agentURLs = append(agentURLs, agentURL)
			k.logger.Debug("discovered agent service",
				"agent", agentName,
				"service", service.Name,
				"namespace", service.Namespace,
				"url", agentURL,
				"component", "k8s_service_discovery")
		}
	}

	k.logger.Info("kubernetes service discovery completed",
		"namespace", k.namespace,
		"discovered_agent_resources", len(agentList.Items),
		"discovered_services", len(agentURLs),
		"component", "k8s_service_discovery")

	return agentURLs, nil
}

// buildServiceURL constructs the URL for an Agent service based on Kubernetes service information
func (k *KubernetesServiceDiscovery) buildServiceURL(service *corev1.Service) string {
	port := k.findA2APort(service)
	if port == 0 {
		k.logger.Warn("no suitable port found for agent service",
			"service", service.Name,
			"namespace", service.Namespace,
			"component", "k8s_service_discovery")
		return ""
	}

	switch service.Spec.Type {
	case corev1.ServiceTypeClusterIP, "":
		return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", service.Name, service.Namespace, port)
	case corev1.ServiceTypeNodePort:
		return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", service.Name, service.Namespace, port)
	case corev1.ServiceTypeLoadBalancer:
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			ingress := service.Status.LoadBalancer.Ingress[0]
			if ingress.IP != "" {
				return fmt.Sprintf("http://%s:%d", ingress.IP, port)
			}
			if ingress.Hostname != "" {
				return fmt.Sprintf("http://%s:%d", ingress.Hostname, port)
			}
		}
		return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", service.Name, service.Namespace, port)
	default:
		k.logger.Warn("unsupported service type for agent discovery",
			"service", service.Name,
			"type", service.Spec.Type,
			"component", "k8s_service_discovery")
		return ""
	}
}

// findA2APort finds the appropriate port for Agent communication from the service spec
func (k *KubernetesServiceDiscovery) findA2APort(service *corev1.Service) int32 {
	for _, port := range service.Spec.Ports {
		portName := strings.ToLower(port.Name)
		if portName == "a2a" || portName == "agent" || portName == "http" {
			return port.Port
		}
	}

	if len(service.Spec.Ports) == 1 {
		return service.Spec.Ports[0].Port
	}

	for _, port := range service.Spec.Ports {
		if port.Port == 8080 {
			return port.Port
		}
	}

	return 0
}

// GetNamespace returns the namespace being monitored for service discovery
func (k *KubernetesServiceDiscovery) GetNamespace() string {
	return k.namespace
}
