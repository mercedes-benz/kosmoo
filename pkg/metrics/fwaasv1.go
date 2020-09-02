// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/fwaas/firewalls"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog"
)

var (
	firewallV1AdminStateUp *prometheus.GaugeVec
	firewallV1Status       *prometheus.GaugeVec

	// according to the nl_constants from https://github.com/openstack/neutron-fwaas/blob/stable/ocata/neutron_fwaas/services/firewall/fwaas_plugin.py
	// we will get the following firewall states
	firewallV1States = []string{"ACTIVE", "DOWN", "ERROR", "INACTIVE", "PENDING_CREATE", "PENDING_UPDATE", "PENDING_DELETE"}

	firewallLabels = []string{"id", "name", "description", "policyID", "projectID"}
)

func registerFWaaSV1Metrics() {
	firewallV1AdminStateUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("firewall_v1_admin_state_up"),
			Help: "Firewall v1 status",
		},
		firewallLabels,
	)
	firewallV1Status = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("firewall_v1_status"),
			Help: "Firewall v1 status",
		},
		append(firewallLabels, "status"),
	)

	prometheus.MustRegister(firewallV1AdminStateUp)
	prometheus.MustRegister(firewallV1Status)
}

// PublishFirewallMetrics makes the list request to the firewall api and
// passes the result to a publish function.
func PublishFirewallMetrics(client *gophercloud.ServiceClient, tenantID string) error {
	// first step: gather the data
	mc := newOpenStackMetric("firewall", "list")
	pages, err := firewalls.List(client, firewalls.ListOpts{}).AllPages()
	if mc.Observe(err) != nil {
		// only warn, maybe the next list will work.
		klog.Warningf("Unable to list firewalls: %v", err)
		return err
	}
	firewallsList, err := firewalls.ExtractFirewalls(pages)
	if err != nil {
		// only warn, maybe the next publish will work.
		klog.Warningf("Unable to extract firewalls: %v", err)
		return err
	}

	// second step: reset the old metrics
	firewallV1AdminStateUp.Reset()
	firewallV1Status.Reset()

	// third step: publish the metrics
	for _, fw := range firewallsList {
		publishFirewallMetric(fw)
	}

	return nil
}

// publishFirewallMetric extracts data from a firewall and exposes the metrics via prometheus
func publishFirewallMetric(fw firewalls.Firewall) {
	labels := []string{fw.ID, fw.Name, fw.Description, fw.PolicyID, fw.ProjectID}
	firewallV1AdminStateUp.WithLabelValues(labels...).Set(boolFloat64(fw.AdminStateUp))

	// create one metric per status
	for _, state := range firewallV1States {
		stateLabels := append(labels, state)
		firewallV1Status.WithLabelValues(stateLabels...).Set(boolFloat64(fw.Status == state))
	}
}
