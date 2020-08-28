<!-- SPDX-License-Identifier: MIT -->
# Prometheus alerts

The following snippet shows some alerts which may be helpful for monitoring a Kubernetes cluster which runs on OpenStack.
The rules may also depend on metrics exposed by [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics).
This alerts assume that the parameter `metrics-prefix` was not changed and still set to the default `kos`

```
  - alert: CinderDiskStuck
    expr: kos_cinder_volume_status{status!~"available|in-use"} == 1
    for: 30m
    labels:
      severity: critical
      team: iaas
    annotations:
      summary: Cinder disk {{ $labels.id }} is stuck in {{ $labels.status }} (node {{ $labels.node }})
      impact: Cinder disk cannot be attached/detached/deleted. Pods may not come up again.
      action: Check PVC {{ $labels.pvc_name }} in {{ $labels.pvc_namespace}} namespace and OpenStack disk.
  - alert: CinderDiskWithoutPV
    expr: |
      kos_cinder_volume_status{name=~"(pvc-.+|kubernetes-dynamic-pvc.+)",pv_name=""} == 1
    for: 30m
    labels:
      severity: warning
      team: caas
    annotations:
      summary: Cinder disk {{ $labels.id }} has no corresponding Kubernetes PV
      impact: The Cinder disk exists but has no corresponding Kubernetes PV. It occupies disk quota and cannot be deleted by CaaS users any more.
      action: Check and delete Cinder volume.
  - alert: CinderDiskAvailableStateMismatch
    expr: kos_cinder_volume_attached_at{status!="in-use"} > 0
    for: 30m
    labels:
      severity: critical
      team: iaas
    annotations:
      summary: Cinder disk {{ $labels.id }} is available but attached to node {{ $labels.node }}
      impact: Cinder disk is in available state, but has attachment information. Reattaching the disk will likely fail.
      action: Check and repair availability/attachment information. Attaching to the node might repair the broken state.
  - alert: CinderStateUnknown
    expr: sum(kos_cinder_volume_status) without(status) == 0
    for: 10m
    labels:
      severity: warning
      team: caas
    annotations:
      summary: Cinder disk {{ $labels.id }} has unknown state
      impact: Aliens invaded. A bit flipped. OpenStack my be broken. At least, we don't know what's going on right now.
      action: Check volume state and extend OpenStack Exporter code if required.
```