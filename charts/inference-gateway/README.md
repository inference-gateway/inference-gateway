# Inference Gateway Helm chart (DEPRECATED)

> [!WARNING]
> **This Helm chart is deprecated and is no longer the supported way to run the Inference Gateway on
> Kubernetes.** Deploy with the
> [Inference Gateway Operator](https://github.com/inference-gateway/operator) instead. This chart is
> kept only for a short migration window and will be removed from the repository (see
> [Removal timeline](#removal-timeline)).

## Why it is deprecated

The Operator is now the supported deployment method for Kubernetes. It reconciles a `Gateway` custom
resource and manages the underlying Deployment, Service, autoscaling (HPA) and north-south routing via
the Kubernetes Gateway API (Envoy Gateway). This replaces the Deployment/Service/Ingress/HPA templates
that this chart rendered, gives a single declarative API surface, and keeps configuration in step with
each gateway release.

## Migrate to the Operator

1. Install the Operator and its prerequisites (Gateway API CRDs and Envoy Gateway). See the
   [Operator repository](https://github.com/inference-gateway/operator) and the
   [documentation](https://docs.inference-gateway.com).
2. Move the values you set on this chart onto a `Gateway` custom resource:
   - Provider API keys → the `inference-gateway-secrets` Secret referenced by the `Gateway`.
   - Environment configuration → the `Gateway` `spec` (providers, telemetry, resources, autoscaling).
   - Ingress/host configuration → `spec.routing` (served by the Gateway API instead of ingress-nginx).
3. Apply the `Gateway` resource and remove the old Helm release once traffic is served by the Operator.

Worked, end-to-end examples for every scenario (basic, hybrid, TLS, authentication, monitoring, MCP and
agent) live under [`examples/kubernetes/`](../../examples/kubernetes/); each one installs the Operator and
applies a `Gateway` custom resource.

## Removal timeline

| Date            | Status                                                                                                   |
| --------------- | -------------------------------------------------------------------------------------------------------- |
| 2026-06         | Chart marked `deprecated: true`. The final published version surfaces the deprecation notice.            |
| 2026-06 – 08    | Deprecation window. No new features; critical/security fixes only. Existing OCI artifacts stay pullable. |
| 2026-09-01      | The `charts/` directory is removed, the CI publish job is removed, and no new versions ship.             |

Chart versions already published to `oci://ghcr.io/inference-gateway/charts/inference-gateway` remain
available after removal but are unmaintained.

> [!NOTE]
> Stopping CI from publishing new chart versions requires removing the `publish_helm_chart` job from
> `.github/workflows/artifacts.yml`. That change is tracked as part of the removal step above.

## Documentation

The Helm → Operator migration notes are tracked in the documentation site alongside the new Kubernetes
deployment guide. See the [documentation](https://docs.inference-gateway.com) and the
[Operator repository](https://github.com/inference-gateway/operator).
