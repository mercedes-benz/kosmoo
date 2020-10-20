// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/common/extensions"
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

	firewallV1Labels = []string{"id", "name", "description", "policyID", "projectID"}
)

func registerFWaaSV1Metrics() {
	firewallV1AdminStateUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("firewall_v1_admin_state_up"),
			Help: "Firewall v1 status",
		},
		firewallV1Labels,
	)
	firewallV1Status = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("firewall_v1_status"),
			Help: "Firewall v1 status",
		},
		append(firewallV1Labels, "status"),
	)

	prometheus.MustRegister(firewallV1AdminStateUp)
	prometheus.MustRegister(firewallV1Status)
}

// PublishFirewallV1Metrics makes the list request to the firewall api and
// passes the result to a publish function. It only does that when the extension
// is available in neutron.
func PublishFirewallV1Metrics(client *gophercloud.ServiceClient, tenantID string) error {
	// check if Neutron extenstion FWaaS v1 is available.
	fwaasV1Extension := extensions.Get(client, "fwaas")
	if fwaasV1Extension.Body != nil {
		return publishFirewallV1Metrics(client, tenantID)
	}

	// reset metrics if fwaas v1 is not available to not publish them anymore
	resetFirewallV1Metrics()
	klog.Info("skipping Firewall metrics as FWaaS v1 is not enabled")
	return nil
}

func publishFirewallV1Metrics(client *gophercloud.ServiceClient, tenantID string) error {
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
	resetFirewallV1Metrics()

	// third step: publish the metrics
	for _, fw := range firewallsList {
		publishFirewallV1Metric(fw)
	}

	return nil
}

// resetFirewallV1Metrics resets the firewall v1 metrics
func resetFirewallV1Metrics() {
	firewallV1AdminStateUp.Reset()
	firewallV1Status.Reset()
}

// publishFirewallV1Metric extracts data from a firewall and exposes the metrics via prometheus
func publishFirewallV1Metric(fw firewalls.Firewall) {
	labels := []string{fw.ID, fw.Name, fw.Description, fw.PolicyID, fw.ProjectID}
	firewallV1AdminStateUp.WithLabelValues(labels...).Set(boolFloat64(fw.AdminStateUp))

	// create one metric per status
	for _, state := range firewallV1States {
		stateLabels := append(labels, state)
		firewallV1Status.WithLabelValues(stateLabels...).Set(boolFloat64(fw.Status == state))
	}
}
