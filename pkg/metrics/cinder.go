// SPDX-License-Identifier: MIT

package metrics

import (
	"fmt"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/extensions/quotasets"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v2/volumes"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

var (
	// labels which get applied to the most metrics
	defaultLabels = []string{"id", "description", "name", "status", "cinder_availability_zone", "volume_type", "pvc_name", "pvc_namespace", "pv_name", "pv_storage_class", "pv_reclaim_policy", "pv_fs_type"}

	// possible cinder states, from https://github.com/openstack/cinder/blob/179ebac5d6d3c15468c0c80c67803fb01a6180a2/cinder/objects/fields.py#L192
	cinderStates = []string{"creating", "available", "deleting", "error", "error-deleting", "error-managing", "managing", "attaching", "in-use", "detaching", "maintenance", "restoring-backup", "error-restoring", "reserved", "awaiting-transfer", "backing-up", "error-backing-up", "error-extending", "downloading", "uploading", "retyping", "extending"}

	cinderQuotaVolumes         *prometheus.GaugeVec
	cinderQuotaVolumesGigabyte *prometheus.GaugeVec
	cinderVolumeCreated        *prometheus.GaugeVec
	cinderVolumeUpdatedAt      *prometheus.GaugeVec
	cinderVolumeStatus         *prometheus.GaugeVec
	cinderVolumeSize           *prometheus.GaugeVec
	cinderVolumeAttachedAt     *prometheus.GaugeVec
)

func registerCinderMetrics() {
	cinderQuotaVolumes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("cinder_quota_volume_disks"),
			Help: "Cinder volume metric (number of volumes)",
		},
		[]string{"quota_type"},
	)
	cinderQuotaVolumesGigabyte = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("cinder_quota_volume_disk_gigabytes"),
			Help: "Cinder volume metric (GB)",
		},
		[]string{"quota_type"},
	)

	cinderVolumeCreated = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("cinder_volume_created_at"),
			Help: "Cinder volume created at",
		},
		defaultLabels,
	)
	cinderVolumeUpdatedAt = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("cinder_volume_updated_at"),
			Help: "Cinder volume updated at",
		},
		defaultLabels,
	)
	cinderVolumeStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("cinder_volume_status"),
			Help: "Cinder volume status",
		},
		defaultLabels,
	)
	cinderVolumeSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("cinder_volume_size"),
			Help: "Cinder volume size",
		},
		defaultLabels,
	)
	cinderVolumeAttachedAt = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: generateName("cinder_volume_attached_at"),
			Help: "Cinder volume attached at",
		},
		append(defaultLabels, "server_id", "device", "hostname"),
	)

	prometheus.MustRegister(cinderQuotaVolumes)
	prometheus.MustRegister(cinderQuotaVolumesGigabyte)
	prometheus.MustRegister(cinderVolumeCreated)
	prometheus.MustRegister(cinderVolumeUpdatedAt)
	prometheus.MustRegister(cinderVolumeSize)
	prometheus.MustRegister(cinderVolumeStatus)
	prometheus.MustRegister(cinderVolumeAttachedAt)
}

// ScrapeCinderMetrics makes the list request to the blockstorage api and passes
// the result to a scrape function.
func ScrapeCinderMetrics(provider *gophercloud.ProviderClient, clientset *kubernetes.Clientset, tenantID string) error {
	// get the cinder client
	client, err := openstack.NewBlockStorageV2(provider, gophercloud.EndpointOpts{Region: "nova"})
	if err != nil {
		return fmt.Errorf("unable to get cinder client: %v", err)
	}

	// get the cinder pvs to add metadata
	pvs, err := getPVsByCinderID(clientset)
	if err != nil {
		return err
	}

	// cinderQuotaVolumes and CinderQuotaVolumesGigabytes are not dynamic and do not need to be reset
	cinderVolumeCreated.Reset()
	cinderVolumeUpdatedAt.Reset()
	cinderVolumeStatus.Reset()
	cinderVolumeSize.Reset()
	cinderVolumeAttachedAt.Reset()

	// get all volumes and scrape them
	pager := volumes.List(client, volumes.ListOpts{})
	err = pager.EachPage(func(page pagination.Page) (bool, error) {
		return scrapeVolumesPage(page, pvs)
	})
	if err != nil {
		// only warn, maybe the next scrape will work.
		klog.Warningf("Unable to scrape volumes: %v", err)
		return err
	}

	q, err := quotasets.GetUsage(client, tenantID).Extract()
	if err != nil {
		// only warn, maybe the next scrape will work.
		klog.Warningf("Unable to scrape quotas: %v", err)
		return err
	}
	scrapeCinderQuotas(q)
	return nil
}

// scrapeCinderQuotas scrapes all cinder related quotas
func scrapeCinderQuotas(q quotasets.QuotaUsageSet) {
	cinderQuotaVolumes.WithLabelValues("in-use").Set(float64(q.Volumes.InUse))
	cinderQuotaVolumes.WithLabelValues("reserved").Set(float64(q.Volumes.Reserved))
	cinderQuotaVolumes.WithLabelValues("limit").Set(float64(q.Volumes.Limit))
	cinderQuotaVolumes.WithLabelValues("allocated").Set(float64(q.Volumes.Allocated))

	cinderQuotaVolumesGigabyte.WithLabelValues("in-use").Set(float64(q.Gigabytes.InUse))
	cinderQuotaVolumesGigabyte.WithLabelValues("reserved").Set(float64(q.Gigabytes.Reserved))
	cinderQuotaVolumesGigabyte.WithLabelValues("limit").Set(float64(q.Gigabytes.Limit))
	cinderQuotaVolumesGigabyte.WithLabelValues("allocated").Set(float64(q.Gigabytes.Allocated))
}

// scrapeVolumesPage iterates over a page, the result of a list request
func scrapeVolumesPage(page pagination.Page, pvs map[string]corev1.PersistentVolume) (bool, error) {
	vList, err := volumes.ExtractVolumes(page)
	if err != nil {
		return false, err
	}

	for _, v := range vList {
		if pv, ok := pvs[v.ID]; ok {
			scrapeVolumeMetrics(v, &pv)
		} else {
			scrapeVolumeMetrics(v, nil)
		}
	}
	return true, nil
}

// scrapeVolumeMetrics extracts data from a volume and exposes the metrics via prometheus
func scrapeVolumeMetrics(v volumes.Volume, pv *corev1.PersistentVolume) {
	labels := []string{v.ID, v.Description, v.Name, v.Status, v.AvailabilityZone, v.VolumeType}

	k8sMetadata := extractK8sMetadata(pv)

	// add the k8s specific labels.
	// We do not need need to check them, because the empty string is totally fine.
	labels = append(labels, k8sMetadata...)

	// set the volume-specific metrics
	cinderVolumeCreated.WithLabelValues(labels...).Set(float64(v.CreatedAt.Unix()))
	cinderVolumeUpdatedAt.WithLabelValues(labels...).Set(float64(v.UpdatedAt.Unix()))
	cinderVolumeSize.WithLabelValues(labels...).Set(float64(v.Size))

	if len(v.Attachments) == 0 {
		l := append(labels, "", "", "")
		cinderVolumeAttachedAt.WithLabelValues(l...).Set(float64(0))
	} else {
		// set the volume-attachment-specific labels
		for _, a := range v.Attachments {
			// TODO: hostname
			l := append(labels, a.ServerID, a.Device, "")
			cinderVolumeAttachedAt.WithLabelValues(l...).Set(float64(a.AttachedAt.Unix()))
		}
	}

	// create one metric per state. If it's the current state it's 1
	for _, status := range cinderStates {
		labels := []string{v.ID, v.Description, v.Name, status, v.AvailabilityZone, v.VolumeType}
		labels = append(labels, k8sMetadata...)
		cinderVolumeStatus.WithLabelValues(labels...).Set(boolFloat64(v.Status == status))
	}
}

func boolFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
