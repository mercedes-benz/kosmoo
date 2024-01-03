// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/loadbalancers"
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/pools"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

var (
	loadbalancerAdminStateUp                 *prometheus.GaugeVec
	loadbalancerStatus                       *prometheus.GaugeVec
	loadbalancerPoolProvisioningStatus       *prometheus.GaugeVec
	loadbalancerPoolMemberProvisioningStatus *prometheus.GaugeVec

	// possible load balancer provisioning states, from https://github.com/openstack/octavia-lib/blob/fe022cdf14604206af783c8a0887c008c48fd053/octavia_lib/common/constants.py#L169
	provisioningStates = []string{"ALLOCATED", "BOOTING", "READY", "ACTIVE", "PENDING_DELETE", "PENDING_UPDATE", "PENDING_CREATE", "DELETED", "ERROR"}

	// https://github.com/gophercloud/gophercloud/blob/0ffab06fc18e06ed9544fe10b385f0a59492fdb5/openstack/loadbalancer/v2/pools/results.go#L100
	// Possible states ACTIVE, PENDING_* or ERROR.
	poolProvisioningStates = []string{"ACTIVE", "PENDING_DELETE", "PENDING_CREATE", "PENDING_UPDATE", "ERROR"}

	loadBalancerLabels = []string{"id", "name", "vip_address", "provider", "port_id"}
	poolLabels         = []string{"pool_id", "pool_name"}
	poolMemberLabels   = []string{"member_id", "member_name"}
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

	loadbalancerPoolProvisioningStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("loadbalancer_pool_provisioning_status"),
			Help: "Load balancer pool provisioning status",
		},
		append(append(loadBalancerLabels, poolLabels...), "pool_provisioning_status"),
	)

	loadbalancerPoolMemberProvisioningStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("loadbalancer_pool_member_provisioning_status"),
			Help: "Load balancer pool member provisioning status",
		},
		append(append(append(loadBalancerLabels, poolLabels...), poolMemberLabels...), "pool_member_provisioning_status"),
	)

	prometheus.MustRegister(loadbalancerAdminStateUp)
	prometheus.MustRegister(loadbalancerStatus)
	prometheus.MustRegister(loadbalancerPoolProvisioningStatus)
	prometheus.MustRegister(loadbalancerPoolMemberProvisioningStatus)
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
	if len(loadBalancerList) == 0 {
		klog.Info("No load balancers found. Skipping load balancer metrics.")
	}

	// second step: reset the old metrics
	loadbalancerAdminStateUp.Reset()
	loadbalancerStatus.Reset()
	loadbalancerPoolProvisioningStatus.Reset()
	loadbalancerPoolMemberProvisioningStatus.Reset()

	// third step: publish the metrics
	for _, lb := range loadBalancerList {
		publishLoadBalancerMetric(lb)

		// for the pools associated with the loadbalancer
		for _, poolWithOnlyId := range lb.Pools {
			pool, err := pools.Get(client, poolWithOnlyId.ID).Extract()
			if err != nil {
				klog.Warningf("Unable to get pool %s: %v", poolWithOnlyId.ID, err)
				continue
			}
			publishPoolStatus(lb, *pool)

			// for the pool members
			for _, memberWithOnlyId := range pool.Members {
				member, err := pools.GetMember(client, pool.ID, memberWithOnlyId.ID).Extract()
				if err != nil {
					klog.Warningf("Unable to get member %s of pool %s: %v", memberWithOnlyId.ID, poolWithOnlyId.ID, err)
					continue
				}
				publishMemberStatus(lb, *pool, *member)
			}
		}
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

func publishPoolStatus(lb loadbalancers.LoadBalancer, pool pools.Pool) {
	labels := []string{lb.ID, lb.Name, lb.VipAddress, lb.Provider, lb.VipPortID, pool.ID, pool.Name}

	for _, state := range poolProvisioningStates {
		stateLabels := append(labels, state)
		loadbalancerPoolProvisioningStatus.WithLabelValues(stateLabels...).Set(boolFloat64(pool.ProvisioningStatus == state))
	}
}

func publishMemberStatus(lb loadbalancers.LoadBalancer, pool pools.Pool, member pools.Member) {
	labels := []string{lb.ID, lb.Name, lb.VipAddress, lb.Provider, lb.VipPortID, pool.ID, pool.Name, member.ID, member.Name}

	for _, state := range poolProvisioningStates {
		stateLabels := append(labels, state)
		loadbalancerPoolMemberProvisioningStatus.WithLabelValues(stateLabels...).Set(boolFloat64(member.ProvisioningStatus == state))
	}
}
