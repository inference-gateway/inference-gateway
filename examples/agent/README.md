# Agent

A bespoke agent workload built on the gateway: a logs-analyzer agent using the
[gateway SDK](https://github.com/inference-gateway/go-sdk) with cluster-wide read
access, pointing at the gateway through the
[Inference Gateway Operator](https://github.com/inference-gateway/operator), with
a deliberately failing deployment for the agent to find. Kubernetes flavour only.
