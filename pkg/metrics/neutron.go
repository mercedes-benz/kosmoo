// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

var (
	neutronFloatingIPStatus    *prometheus.GaugeVec
	neutronFloatingIPCreated   *prometheus.GaugeVec
	neutronFloatingIPUpdatedAt *prometheus.GaugeVec

	// Status from https://docs.openstack.org/api-ref/network/v2/index.html?expanded=show-floating-ip-details-detail#show-floating-ip-details
	floatingIpStatus = []string{"ACTIVE", "DOWN", "ERROR"}

	floatingIPLabels = []string{"id", "floating_ip", "fixed_ip", "port_id"}
)

func registerNeutronMetrics() {
	neutronFloatingIPStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("neutron_floating_ip_status"),
			Help: "Neutron floating ip status",
		},
		append(floatingIPLabels, "status"),
	)
	neutronFloatingIPCreated = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("neutron_floatingip_created_at"),
			Help: "Neutron floating ip created at",
		},
		floatingIPLabels,
	)
	neutronFloatingIPUpdatedAt = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("neutron_floatingip_updated_at"),
			Help: "Neutron floating ip updated at",
		},
		floatingIPLabels,
	)

	prometheus.MustRegister(neutronFloatingIPStatus)
	prometheus.MustRegister(neutronFloatingIPCreated)
	prometheus.MustRegister(neutronFloatingIPUpdatedAt)
}

// PublishNeutronMetrics makes the list request to the neutron api and passes
// the result to a publish function.
func PublishNeutronMetrics(neutronClient *gophercloud.ServiceClient, tenantID string) error {
	// first step: gather the data
	mc := newOpenStackMetric("floating_ip", "list")
	pages, err := floatingips.List(neutronClient, floatingips.ListOpts{}).AllPages()
	if mc.Observe(err) != nil {
		// only warn, maybe the next list will work.
		klog.Warningf("Unable to list floating ips: %v", err)
		return err
	}
	floatingIPList, err := floatingips.ExtractFloatingIPs(pages)
	if err != nil {
		// only warn, maybe the next extract will work.
		klog.Warningf("Unable to extract floating ips: %v", err)
		return err
	}

	// second step: reset the old metrics
	neutronFloatingIPStatus.Reset()
	neutronFloatingIPUpdatedAt.Reset()

	// third step: publish the metrics
	for _, fip := range floatingIPList {
		publishFloatingIPMetric(fip)
	}

	return nil
}

// publishFloatingIPMetric extracts data from a floating ip and exposes the metrics via prometheus
func publishFloatingIPMetric(fip floatingips.FloatingIP) {
	labels := []string{fip.ID, fip.FloatingIP, fip.FixedIP, fip.PortID}

	neutronFloatingIPCreated.WithLabelValues(labels...).Set(float64(fip.CreatedAt.Unix()))
	neutronFloatingIPUpdatedAt.WithLabelValues(labels...).Set(float64(fip.UpdatedAt.Unix()))

	for _, status := range floatingIpStatus {
		statusLabels := append(labels, status)
		neutronFloatingIPStatus.WithLabelValues(statusLabels...).Set(boolFloat64(fip.Status == status))
	}

}
