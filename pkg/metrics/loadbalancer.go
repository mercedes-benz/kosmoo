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

	loadBalancerLabels = []string{"id", "vip_address", "provider", "port_id"}
)

func registerLoadBalancerMetrics() {
	loadbalancerAdminStateUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("loadbalancer_admin_state_up"),
			Help: "Load balancer status",
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

// ScrapeLoadBalancerMetrics makes the list request to the load balancer api and
// passes the result to a scrape function.
func ScrapeLoadBalancerMetrics(client *gophercloud.ServiceClient, tenantID string) error {
	// first step: gather the data
	pages, err := loadbalancers.List(client, loadbalancers.ListOpts{}).AllPages()
	if err != nil {
		// only warn, maybe the next scrape will work.
		klog.Warningf("Unable to scrape floating ips: %v", err)
		return err
	}
	loadBalancerList, err := loadbalancers.ExtractLoadBalancers(pages)
	if err != nil {
		// only warn, maybe the next scrape will work.
		klog.Warningf("Unable to scrape load balancers: %v", err)
		return err
	}

	// second step: reset the old metrics
	loadbalancerStatus.Reset()

	// third step: publish the metrics
	for _, lb := range loadBalancerList {
		scrapeLoadBalancerMetric(lb)
	}

	return nil
}

// scrapeLoadBalancerMetric extracts data from a floating ip and exposes the metrics via prometheus
func scrapeLoadBalancerMetric(lb loadbalancers.LoadBalancer) {
	labels := []string{lb.ID, lb.VipAddress, lb.Provider, lb.VipPortID}

	loadbalancerAdminStateUp.WithLabelValues(labels...).Set(boolFloat64(lb.AdminStateUp))

	// create one metric per provisioning status
	for _, state := range provisioningStates {
		stateLabels := append(labels, state)
		loadbalancerStatus.WithLabelValues(stateLabels...).Set(boolFloat64(lb.ProvisioningStatus == state))
	}
}
