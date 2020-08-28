// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog"
)

var (
	neutronFloatingIPStatus    *prometheus.GaugeVec
	neutronFloatingIPCreated   *prometheus.GaugeVec
	neutronFloatingIPUpdatedAt *prometheus.GaugeVec

	floatingIPLabels = []string{"id", "floating_ip", "fixed_ip", "port_id"}
)

func registerNeutronMetrics() {
	neutronFloatingIPStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("neutron_floating_ip_status"),
			Help: "Neutron floating ip status",
		},
		floatingIPLabels,
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

// ScrapeNeutronMetrics makes the list request to the neutron api and passes
// the result to a scrape function.
func ScrapeNeutronMetrics(neutronClient *gophercloud.ServiceClient, tenantID string) error {
	// first step: gather the data
	mc := newOpenStackMetric("floating_ip", "list")
	pages, err := floatingips.List(neutronClient, floatingips.ListOpts{}).AllPages()
	if mc.Observe(err) != nil {
		// only warn, maybe the next scrape will work.
		klog.Warningf("Unable to scrape floating ips: %v", err)
		return err
	}
	floatingIPList, err := floatingips.ExtractFloatingIPs(pages)
	if err != nil {
		// only warn, maybe the next scrape will work.
		klog.Warningf("Unable to scrape floating ips: %v", err)
		return err
	}

	// second step: reset the old metrics
	neutronFloatingIPStatus.Reset()

	// third step: publish the metrics
	for _, fip := range floatingIPList {
		scrapeFloatingIPMetric(fip)
	}

	return nil
}

// scrapeFloatingIPMetric extracts data from a floating ip and exposes the metrics via prometheus
func scrapeFloatingIPMetric(fip floatingips.FloatingIP) {
	labels := []string{fip.ID, fip.FloatingIP, fip.FixedIP, fip.PortID}

	neutronFloatingIPStatus.WithLabelValues(labels...).Set(1)
	neutronFloatingIPCreated.WithLabelValues(labels...).Set(float64(fip.CreatedAt.Unix()))
	neutronFloatingIPUpdatedAt.WithLabelValues(labels...).Set(float64(fip.UpdatedAt.Unix()))
}
