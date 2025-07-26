package a2a

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesServiceDiscovery handles Kubernetes-based service discovery for A2A agents
type KubernetesServiceDiscovery struct {
	client        kubernetes.Interface
	namespace     string
	labelSelector string
	logger        logger.Logger
	config        *config.A2AConfig
}

// IsKubernetesEnvironment detects if the application is running in a Kubernetes environment
func IsKubernetesEnvironment() bool {
	// Check for service account token (most reliable indicator)
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		return true
	}

	// Check for KUBERNETES_SERVICE_HOST environment variable
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	// Check for kubeconfig file (development/external usage)
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		if _, err := os.Stat(kubeconfig); err == nil {
			return true
		}
	}

	// Check for default kubeconfig location
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

	namespace := cfg.ServiceDiscoveryNamespace
	if namespace == "" {
		// Try to get current namespace from service account
		if namespaceBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			namespace = strings.TrimSpace(string(namespaceBytes))
		}
		// Default to "default" namespace if still empty
		if namespace == "" {
			namespace = "default"
		}
	}

	labelSelector := cfg.ServiceDiscoveryLabelSelector
	if labelSelector == "" {
		labelSelector = "inference-gateway.com/a2a-agent=true"
	}

	return &KubernetesServiceDiscovery{
		client:        client,
		namespace:     namespace,
		labelSelector: labelSelector,
		logger:        logger,
		config:        cfg,
	}, nil
}

// createKubernetesClient creates a Kubernetes client using in-cluster config or kubeconfig
func createKubernetesClient() (kubernetes.Interface, error) {
	// Try in-cluster config first (for pods running in Kubernetes)
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig (for development/external usage)
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

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return client, nil
}

// DiscoverA2AServices discovers A2A services in the Kubernetes cluster
func (k *KubernetesServiceDiscovery) DiscoverA2AServices(ctx context.Context) ([]string, error) {
	services, err := k.client.CoreV1().Services(k.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: k.labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	var agentURLs []string
	for _, service := range services.Items {
		agentURL := k.buildServiceURL(&service)
		if agentURL != "" {
			agentURLs = append(agentURLs, agentURL)
			k.logger.Debug("discovered a2a service",
				"service", service.Name,
				"namespace", service.Namespace,
				"url", agentURL,
				"component", "k8s_service_discovery")
		}
	}

	k.logger.Info("kubernetes service discovery completed",
		"namespace", k.namespace,
		"label_selector", k.labelSelector,
		"discovered_services", len(agentURLs),
		"component", "k8s_service_discovery")

	return agentURLs, nil
}

// buildServiceURL constructs the URL for an A2A service based on Kubernetes service information
func (k *KubernetesServiceDiscovery) buildServiceURL(service *corev1.Service) string {
	// Determine the appropriate port for A2A communication
	port := k.findA2APort(service)
	if port == 0 {
		k.logger.Warn("no suitable port found for a2a service",
			"service", service.Name,
			"namespace", service.Namespace,
			"component", "k8s_service_discovery")
		return ""
	}

	// Check for custom URL annotation first
	if customURL, exists := service.Annotations["inference-gateway.com/a2a-url"]; exists && customURL != "" {
		return customURL
	}

	// Build URL based on service type and configuration
	switch service.Spec.Type {
	case corev1.ServiceTypeClusterIP, "":
		// Use internal cluster DNS name
		return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", service.Name, service.Namespace, port)
	case corev1.ServiceTypeNodePort:
		// For NodePort, we need the node IP or can use the service DNS name with the port
		return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", service.Name, service.Namespace, port)
	case corev1.ServiceTypeLoadBalancer:
		// For LoadBalancer, try to use the external IP if available
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			ingress := service.Status.LoadBalancer.Ingress[0]
			if ingress.IP != "" {
				return fmt.Sprintf("http://%s:%d", ingress.IP, port)
			}
			if ingress.Hostname != "" {
				return fmt.Sprintf("http://%s:%d", ingress.Hostname, port)
			}
		}
		// Fall back to cluster DNS if external IP not available yet
		return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", service.Name, service.Namespace, port)
	default:
		k.logger.Warn("unsupported service type for a2a discovery",
			"service", service.Name,
			"type", service.Spec.Type,
			"component", "k8s_service_discovery")
		return ""
	}
}

// findA2APort finds the appropriate port for A2A communication from the service spec
func (k *KubernetesServiceDiscovery) findA2APort(service *corev1.Service) int32 {
	// Look for port with specific name patterns
	for _, port := range service.Spec.Ports {
		portName := strings.ToLower(port.Name)
		if portName == "a2a" || portName == "agent" || portName == "http" {
			return port.Port
		}
	}

	// Look for port with A2A annotation
	if portStr, exists := service.Annotations["inference-gateway.com/a2a-port"]; exists {
		for _, port := range service.Spec.Ports {
			if fmt.Sprintf("%d", port.Port) == portStr {
				return port.Port
			}
		}
	}

	// Fall back to the first port if only one port is defined
	if len(service.Spec.Ports) == 1 {
		return service.Spec.Ports[0].Port
	}

	// Default to common A2A port (8080) if it exists
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

// GetLabelSelector returns the label selector used for service discovery
func (k *KubernetesServiceDiscovery) GetLabelSelector() string {
	return k.labelSelector
}
