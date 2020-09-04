// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog"
)

var (
	serverStatus                *prometheus.GaugeVec
	serverVolumeAttachmentCount *prometheus.GaugeVec

	// possible server states, from https://github.com/openstack/nova/blob/master/nova/objects/fields.py#L949
	states = []string{"ACTIVE", "BUILDING", "PAUSED", "SUSPENDED", "STOPPED", "RESCUED", "RESIZED", "SOFT_DELETED", "DELETED", "ERROR", "SHELVED", "SHELVED_OFFLOADED"}

	serverLabels = []string{"id", "name"}
)

func registerServerMetrics() {
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

	prometheus.MustRegister(serverStatus)
	prometheus.MustRegister(serverVolumeAttachmentCount)
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

	// third step: publish the metrics
	for _, srv := range serversList {
		publishServerMetric(srv)
	}

	return nil
}

// publishServerMetric extracts data from a server and exposes the metrics via prometheus
func publishServerMetric(srv servers.Server) {
	labels := []string{srv.ID, srv.Name}

	serverVolumeAttachmentCount.WithLabelValues(labels...).Set(float64(len(srv.AttachedVolumes)))

	// create one metric per status
	for _, state := range states {
		stateLabels := append(labels, state)
		serverStatus.WithLabelValues(stateLabels...).Set(boolFloat64(srv.Status == state))
	}
}
