// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/loadbalancers"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog"
)

var (
	loadbalancerAdminStateUp *prometheus.GaugeVec
	loadbalancerStatus       *prometheus.GaugeVec

	// possible load balancer provisioning states, from https://github.com/openstack/octavia-lib/blob/fe022cdf14604206af783c8a0887c008c48fd053/octavia_lib/common/constants.py#L169
	provisioningStates = []string{"ALLOCATED", "BOOTING", "READY", "ACTIVE", "PENDING_DELETE", "PENDING_UPDATE", "PENDING_CREATE", "DELETED", "ERROR"}

	loadBalancerLabels = []string{"id", "name", "vip_address", "provider", "port_id"}
)

func registerLoadBalancerMetrics() {
	loadbalancerAdminStateUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("loadbalancer_admin_state_up"),
			Help: "Load balancer admin state up",
		},
		loadBalancerLabels,
	)
	loadbalancerStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("loadbalancer_provisioning_status"),
			Help: "Load balancer status",
		},
		append(loadBalancerLabels, "provisioning_status"),
	)

	prometheus.MustRegister(loadbalancerAdminStateUp)
	prometheus.MustRegister(loadbalancerStatus)
}

// PublishLoadBalancerMetrics makes the list request to the load balancer api and
// passes the result to a publish function.
func PublishLoadBalancerMetrics(client *gophercloud.ServiceClient, tenantID string) error {
	// first step: gather the data
	mc := newOpenStackMetric("loadbalancer", "list")
	pages, err := loadbalancers.List(client, loadbalancers.ListOpts{}).AllPages()
	if mc.Observe(err) != nil {
		// only warn, maybe the next list will work.
		klog.Warningf("Unable to list load balancers: %v", err)
		return err
	}
	loadBalancerList, err := loadbalancers.ExtractLoadBalancers(pages)
	if err != nil {
		// only warn, maybe the next publish will work.
		klog.Warningf("Unable to extract load balancers: %v", err)
		return err
	}

	// second step: reset the old metrics
	loadbalancerAdminStateUp.Reset()
	loadbalancerStatus.Reset()

	// third step: publish the metrics
	for _, lb := range loadBalancerList {
		publishLoadBalancerMetric(lb)
	}

	return nil
}

// publishLoadBalancerMetric extracts data from a load balancer and exposes the metrics via prometheus
func publishLoadBalancerMetric(lb loadbalancers.LoadBalancer) {
	labels := []string{lb.ID, lb.Name, lb.VipAddress, lb.Provider, lb.VipPortID}

	loadbalancerAdminStateUp.WithLabelValues(labels...).Set(boolFloat64(lb.AdminStateUp))

	// create one metric per provisioning status
	for _, state := range provisioningStates {
		stateLabels := append(labels, state)
		loadbalancerStatus.WithLabelValues(stateLabels...).Set(boolFloat64(lb.ProvisioningStatus == state))
	}
}
