// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/common/extensions"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/fwaas_v2/groups"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog"
)

var (
	firewallV2GroupAdminStateUp *prometheus.GaugeVec
	firewallV2GroupStatus       *prometheus.GaugeVec

	// according to the nl_constants from https://github.com/openstack/neutron-fwaas/blob/stable/ocata/neutron_fwaas/services/firewall/fwaas_plugin.py
	// we will get the following firewall states
	firewallV2States = []string{"ACTIVE", "DOWN", "ERROR", "INACTIVE", "PENDING_CREATE", "PENDING_DELETE", "PENDING_UPDATE"}

	firewallV2Labels = []string{"id", "name", "description", "ingressPolicyID", "egressPolicyID", "projectID"}
)

func registerFWaaSV2Metrics() {
	firewallV2GroupAdminStateUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("firewall_v2_group_admin_state_up"),
			Help: "Firewall v2 status",
		},
		firewallV2Labels,
	)
	firewallV2GroupStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("firewall_v2_group_status"),
			Help: "Firewall v2 status",
		},
		append(firewallV2Labels, "status"),
	)

	prometheus.MustRegister(firewallV2GroupAdminStateUp)
	prometheus.MustRegister(firewallV2GroupStatus)
}

// PublishFirewallV1Metrics makes the list request to the firewall api and
// passes the result to a publish function. It only does that when the extension
// is available in neutron.
func PublishFirewallV2Metrics(client *gophercloud.ServiceClient, tenantID string) error {
	// check if Neutron extenstion FWaaS v1 is available.
	fwaasV1Extension := extensions.Get(client, "fwaas_v2")
	if fwaasV1Extension.Body != nil {
		return publishFirewallV2Metrics(client, tenantID)
	}

	// reset metrics if fwaas v1 is not available to not publish them anymore
	resetFirewallV2Metrics()
	klog.Info("skipping Firewall metrics as FWaaS v1 is not enabled")
	return nil
}

func publishFirewallV2Metrics(client *gophercloud.ServiceClient, tenantID string) error {
	// first step: gather the data
	mc := newOpenStackMetric("firewallv2_group", "list")
	pages, err := groups.List(client, groups.ListOpts{}).AllPages()
	if mc.Observe(err) != nil {
		// only warn, maybe the next list will work.
		klog.Warningf("Unable to list firewall groups: %v", err)
		return err
	}
	groupsList, err := groups.ExtractGroups(pages)
	if err != nil {
		// only warn, maybe the next publish will work.
		klog.Warningf("Unable to extract firewall groups: %v", err)
		return err
	}

	// second step: reset the old metrics
	resetFirewallV2Metrics()

	// third step: publish the metrics
	for _, group := range groupsList {
		if group.Name == "default" {
			continue
		}
		publishFirewallV2GroupMetric(group)
	}

	return nil
}

// resetFirewallV2Metrics resets the firewall v1 metrics
func resetFirewallV2Metrics() {
	firewallV2GroupAdminStateUp.Reset()
	firewallV2GroupStatus.Reset()
}

// publishFirewallV2GroupMetric extracts data from a firewall and exposes the metrics via prometheus
func publishFirewallV2GroupMetric(group groups.Group) {
	labels := []string{group.ID, group.Name, group.Description, group.IngressFirewallPolicyID, group.EgressFirewallPolicyID, group.ProjectID}
	firewallV2GroupAdminStateUp.WithLabelValues(labels...).Set(boolFloat64(group.AdminStateUp))

	// create one metric per status
	for _, state := range firewallV2States {
		stateLabels := append(labels, state)
		firewallV2GroupStatus.WithLabelValues(stateLabels...).Set(boolFloat64(group.Status == state))
	}
}
