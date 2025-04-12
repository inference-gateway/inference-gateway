package kubegen

// Package kubegen provides functionality for generating Helm templates and values
// for Kubernetes ConfigMaps and Secrets from OpenAPI specifications.

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/inference-gateway/inference-gateway/internal/openapi"
)

// GenerateHelmSecret generates a Helm template for a Kubernetes Secret from an OpenAPI spec.
// The generated template uses Helm values (.Values.secrets) for configuration.
func GenerateHelmSecret(filePath string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	tmpl := `apiVersion: v1
kind: Secret
metadata:
  name: {{ "{{" }} include "inference-gateway.fullname" . {{ "}}" }}-defaults
  labels:
    {{ "{{-" }} include "inference-gateway.labels" . | nindent 4 {{ "}}" }}
stringData:
  {{- range $section := .Sections }}
  {{- range $name, $section := $section }}
  {{- if or (eq $name "oidc") (eq $name "providers") }}
  # {{ $section.Title }}
  {{- range $setting := $section.Settings }}
  {{- if $setting.Secret }}
  {{ $setting.Env }}: ""
  {{- end }}
  {{- end }}
  {{- end -}}
  {{- end -}}
  {{- end }}
`

	t, err := template.New("helm-secret").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	}).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	data := struct {
		Sections  []map[string]openapi.Section
		Providers map[string]openapi.ProviderConfig
	}{
		Sections:  schema.Components.Schemas.Config.XConfig.Sections,
		Providers: schema.Components.Schemas.Provider.XProviderConfigs,
	}

	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// GenerateHelmValues generates the values.yaml content from an OpenAPI spec
func GenerateHelmValues(filePath string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	tmpl := `# Default values for inference-gateway.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

replicaCount: 1

image:
  repository: ghcr.io/inference-gateway/inference-gateway
  pullPolicy: IfNotPresent
  tag: ""

monitoring:
  enabled: false
  metricsPort: 9464
  namespaceSelector:
    matchNames:
      - monitoring

# Secrets and configurations references
envFrom:
  configMapRef:
  secretRef:

# This is for the secrets for pulling an image from a private repository more information can be found here: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
imagePullSecrets: []
# This is to override the chart name.
nameOverride: ""
fullnameOverride: ""

# This section builds out the service account more information can be found here: https://kubernetes.io/docs/concepts/security/service-accounts/
serviceAccount:
  # Specifies whether a service account should be created
  create: true
  # Automatically mount a ServiceAccount's API credentials?
  automount: true
  # Annotations to add to the service account
  annotations: {}
  # The name of the service account to use.
  # If not set and create is true, a name is generated using the fullname template
  name: ""

# This is for setting Kubernetes Annotations to a Pod.
# For more information checkout: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
podAnnotations: {}
# This is for setting Kubernetes Labels to a Pod.
# For more information checkout: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
podLabels: {}

podSecurityContext:
  {}
  # fsGroup: 2000

securityContext:
  {}
  # capabilities:
  #   drop:
  #   - ALL
  # readOnlyRootFilesystem: true
  # runAsNonRoot: true
  # runAsUser: 1000

# This is for setting up a service more information can be found here: https://kubernetes.io/docs/concepts/services-networking/service/
service:
  # This sets the service type more information can be found here: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
  type: ClusterIP
  # This sets the ports more information can be found here: https://kubernetes.io/docs/concepts/services-networking/service/#field-spec-ports
  port: 8080

# This block is for setting up the ingress for more information can be found here: https://kubernetes.io/docs/concepts/services-networking/ingress/
ingress:
  enabled: false
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: "selfsigned-issuer"
  hosts:
    - host: api.inference-gateway.local
      paths:
        - path: /
          pathType: ImplementationSpecific
  tls:
    enabled: false
    # Hosts can be specified as either:
    # hosts: api.inference-gateway.local  # Single host
    # or
    hosts:
      - api.inference-gateway.local
    secretName: api-inference-gateway-tls

resources:
  limits:
    cpu: 200m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi

# This is to setup the liveness and readiness probes more information can be found here: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/
livenessProbe:
  httpGet:
    path: /health
    port: http
readinessProbe:
  httpGet:
    path: /health
    port: http

# This section is for setting up autoscaling more information can be found here: https://kubernetes.io/docs/concepts/workloads/autoscaling/
autoscaling:
  enabled: false
  minReplicas: 1
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80
  targetMemoryUtilizationPercentage: 80

# Additional volumes on the output Deployment definition.
volumes: []
# - name: foo
#   secret:
#     secretName: mysecret
#     optional: false

# Additional volumeMounts on the output Deployment definition.
volumeMounts: []
# - name: foo
#   mountPath: "/etc/foo"
#   readOnly: true

nodeSelector: {}

tolerations: []

affinity: {}

# Extra environment variables to add to the deployment
extraEnv: []
# - name: EXAMPLE_VAR
#   value: "example-value"

config:
  {{- range $section := .Sections }}
  {{- range $name, $section := $section }}
  # {{ $section.Title }}
  {{- range $setting := $section.Settings }}
  {{- if not $setting.Secret }}
  {{ $setting.Env }}: {{ printf "%q" $setting.Default }}
  {{- end }}
  {{- end }}
  {{- end -}}
  {{- end }}
`

	t, err := template.New("helm-values").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	}).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	data := struct {
		Sections []map[string]openapi.Section
	}{
		Sections: schema.Components.Schemas.Config.XConfig.Sections,
	}

	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func GenerateHelmConfigMap(filePath string, oas string) error {
	schema, err := openapi.Read(oas)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec: %w", err)
	}

	tmpl := `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ "{{" }} include "inference-gateway.fullname" . {{ "}}" }}-defaults
  labels:
    {{ "{{-" }} include "inference-gateway.labels" . | nindent 4 {{ "}}" }}
data:
  {{- range $section := .Sections }}
  {{- range $name, $section := $section }}
  # {{ $section.Title }}
  {{- range $setting := $section.Settings }}
  {{- if not $setting.Secret }}
  {{ $setting.Env }}: {{ printf "{{ .Values.config.%s | quote }}" $setting.Env }}
  {{- end }}
  {{- end }}
  {{- end -}}
  {{- end }}
`

	t, err := template.New("helm-configmap").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	}).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	data := struct {
		Sections  []map[string]openapi.Section
		Providers map[string]openapi.ProviderConfig
	}{
		Sections:  schema.Components.Schemas.Config.XConfig.Sections,
		Providers: schema.Components.Schemas.Provider.XProviderConfigs,
	}

	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
