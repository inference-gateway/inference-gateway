# Hybrid

Several providers behind one gateway: cloud models next to local ones (Ollama,
llama.cpp), selected per request by the `provider/model` prefix. Route a coding
request to a cloud model and a simple one to localhost in the same session.

## Flavours

- [docker-compose](docker-compose/README.md) - cloud providers plus local Ollama
  and llama.cpp services
- [kubernetes](kubernetes/README.md) - the local providers deployed as
  in-cluster workloads (Ollama and llama.cpp manifests included) with the
  [Inference Gateway Operator](https://github.com/inference-gateway/operator)
