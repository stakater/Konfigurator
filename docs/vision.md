# Konfigurator — Vision & value proposition

> **Status: 📝 Draft.** This is reconstructed from the README and the code during the
> 2026-11-17 baseline review. It hasn't been agreed by the product owner. Confirm it or
> rewrite it before a story is approved against it.

## Problem
Some apps need configuration that depends on *other live cluster objects*. For example, a log
shipper needs per-pod parsing rules, or a proxy needs the current set of services/ingresses.
Writing that config by hand goes stale as soon as pods churn.

## Vision
A Kubernetes operator that keeps such configuration **continuously derived from the live
cluster state**. The author writes a Go template once. Konfigurator renders it against
Pods/Services/Ingresses into a ConfigMap or Secret and mounts the result into the consuming
workload.

## Value proposition
- **For platform teams** running shared agents (fluentd is the canonical example, see
  `examples/`): config follows the workloads automatically, with no hand-edited ConfigMaps.
- **Safe on shared clusters** *(aspiration, not true today; see review S1–S4)*: a tenant's
  template sees only what the tenant may see. The operator holds least privilege.
- **Predictable:** rendered output reflects the live state, never partial or stale state.
  Failures show up in `.status`, not only in operator logs *(also not true today; see
  B1–B5)*.

## Non-goals (proposed)
- A general GitOps/config-management tool. Konfigurator only derives config from cluster
  state.
- Rendering arbitrary object kinds. The context is limited to what templates need, and every
  addition is a deliberate story.
