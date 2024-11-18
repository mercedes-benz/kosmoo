// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/quotasets"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

var (
	computeQuotaCores           *prometheus.GaugeVec
	computeQuotaFloatingIPs     *prometheus.GaugeVec
	computeQuotaInstances       *prometheus.GaugeVec
	computeQuotaRAM             *prometheus.GaugeVec
	computeQuotaServerGroupMembers *prometheus.GaugeVec
	serverStatus                *prometheus.GaugeVec
	serverVolumeAttachment      *prometheus.GaugeVec
	serverVolumeAttachmentCount *prometheus.GaugeVec
	
	// possible server states, from https://github.com/openstack/nova/blob/master/nova/objects/fields.py#L949
	states = []string{"ACTIVE", "BUILDING", "PAUSED", "SUSPENDED", "STOPPED", "RESCUED", "RESIZED", "SOFT_DELETED", "DELETED", "ERROR", "SHELVED", "SHELVED_OFFLOADED"}

	serverLabels = []string{"id", "name"}
)

func registerServerMetrics() {
	computeQuotaCores = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("compute_quota_cores"),
			Help: "Number of instance cores allowed",
		},
		[]string{"quota_type"},
	)
	computeQuotaFloatingIPs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("compute_quota_floating_ips"),
			Help: "Number of floating IPs allowed",
		},
		[]string{"quota_type"},
	)
	computeQuotaInstances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("compute_quota_instances"),
			Help: "Number of instances (servers) allowed",
		},
		[]string{"quota_type"},
	)
	computeQuotaRAM = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("compute_quota_ram_megabytes"),
			Help: "RAM (in MB) allowed",
		},
		[]string{"quota_type"},
	)
	computeQuotaServerGroupMembers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("compute_quota_server_group_members"),
			Help: "Number of members allowed per server group",
		},
		[]string{"quota_type"},
	)
	serverStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("server_status"),
			Help: "Server status",
		},
		append(serverLabels, "status"),
	)
	serverVolumeAttachmentCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("server_volume_attachment_count"),
			Help: "Server volume attachment count",
		},
		serverLabels,
	)
	serverVolumeAttachment = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("server_volume_attachment"),
			Help: "Server volume attachment",
		},
		append(serverLabels, "volume_id"),
	)

	prometheus.MustRegister(computeQuotaCores)
	prometheus.MustRegister(computeQuotaFloatingIPs)
	prometheus.MustRegister(computeQuotaInstances)
	prometheus.MustRegister(computeQuotaRAM)
	prometheus.MustRegister(computeQuotaServerGroupMembers)
	prometheus.MustRegister(serverStatus)
	prometheus.MustRegister(serverVolumeAttachmentCount)
	prometheus.MustRegister(serverVolumeAttachment)
}

// PublishServerMetrics makes the list request to the server api and
// passes the result to a publish function.
func PublishServerMetrics(client *gophercloud.ServiceClient, tenantID string) error {
	// first step: gather the data
	mc := newOpenStackMetric("server", "list")
	pages, err := servers.List(client, servers.ListOpts{}).AllPages()
	if mc.Observe(err) != nil {
		// only warn, maybe the next list will work.
		klog.Warningf("Unable to list servers: %v", err)
		return err
	}
	serversList, err := servers.ExtractServers(pages)
	if err != nil {
		// only warn, maybe the next publish will work.
		klog.Warningf("Unable to extract servers: %v", err)
		return err
	}

	// second step: reset the old metrics
	serverStatus.Reset()
	serverVolumeAttachmentCount.Reset()
	serverVolumeAttachment.Reset()

	// third step: publish the metrics
	for _, srv := range serversList {
		publishServerMetric(srv)
	}

	// Get compute quotas from OpenStack.
	mc = newOpenStackMetric("compute_quotasets_detail", "get")
	quotas, err := quotasets.GetDetail(client, tenantID).Extract()
	if mc.Observe(err) != nil {
		// only warn, maybe the next get will work.
		klog.Warningf("Unable to get compute quotas: %v", err)
		return err
	}
	publishComputeQuotaMetrics(quotas)

	return nil
}

// publishServerMetric extracts data from a server and exposes the metrics via prometheus
func publishServerMetric(srv servers.Server) {
	labels := []string{srv.ID, srv.Name}

	serverVolumeAttachmentCount.WithLabelValues(labels...).Set(float64(len(srv.AttachedVolumes)))
	for _, attachedVolumeID := range srv.AttachedVolumes {
		serverVolumeAttachment.WithLabelValues(append(labels, attachedVolumeID.ID)...).Set(1)
	}

	// create one metric per status
	for _, state := range states {
		stateLabels := append(labels, state)
		serverStatus.WithLabelValues(stateLabels...).Set(boolFloat64(srv.Status == state))
	}
}

// publishComputeQuotaMetrics publishes all compute related quotas
func publishComputeQuotaMetrics(q quotasets.QuotaDetailSet) {
	computeQuotaCores.WithLabelValues("in-use").Set(float64(q.Cores.InUse))
	computeQuotaCores.WithLabelValues("reserved").Set(float64(q.Cores.Reserved))
	computeQuotaCores.WithLabelValues("limit").Set(float64(q.Cores.Limit))

	computeQuotaFloatingIPs.WithLabelValues("in-use").Set(float64(q.FloatingIPs.InUse))
	computeQuotaFloatingIPs.WithLabelValues("reserved").Set(float64(q.FloatingIPs.Reserved))
	computeQuotaFloatingIPs.WithLabelValues("limit").Set(float64(q.FloatingIPs.Limit))

	computeQuotaInstances.WithLabelValues("in-use").Set(float64(q.Instances.InUse))
	computeQuotaInstances.WithLabelValues("reserved").Set(float64(q.Instances.Reserved))
	computeQuotaInstances.WithLabelValues("limit").Set(float64(q.Instances.Limit))

	computeQuotaServerGroupMembers.WithLabelValues("in-use").Set(float64(q.ServerGroupMembers.InUse))
	computeQuotaServerGroupMembers.WithLabelValues("reserved").Set(float64(q.ServerGroupMembers.Reserved))
	computeQuotaServerGroupMembers.WithLabelValues("limit").Set(float64(q.ServerGroupMembers.Limit))

	computeQuotaRAM.WithLabelValues("in-use").Set(float64(q.RAM.InUse))
	computeQuotaRAM.WithLabelValues("reserved").Set(float64(q.RAM.Reserved))
	computeQuotaRAM.WithLabelValues("limit").Set(float64(q.RAM.Limit))
}
