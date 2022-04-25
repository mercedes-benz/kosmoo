// SPDX-License-Identifier: MIT

package metrics

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/extensions/quotasets"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v2/volumes"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	// labels which get applied to the most metrics
	defaultLabels = []string{"id", "description", "name", "status", "cinder_availability_zone", "volume_type", "pvc_name", "pvc_namespace", "pv_name", "pv_storage_class", "pv_reclaim_policy", "pv_fs_type"}

	// possible cinder states, from https://github.com/openstack/cinder/blob/master/cinder/objects/fields.py#L168
	cinderStates = []string{"creating", "available", "deleting", "error", "error_deleting", "error_managing", "managing", "attaching", "in-use", "detaching", "maintenance", "restoring-backup", "error_restoring", "reserved", "awaiting-transfer", "backing-up", "error_backing-up", "error_extending", "downloading", "uploading", "retyping", "extending"}

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

// PublishCinderMetrics makes the list request to the blockstorage api and passes
// the result to a publish function.
func PublishCinderMetrics(client *gophercloud.ServiceClient, clientset *kubernetes.Clientset, tenantID string) error {
	// first step: gather the data

	// get the cinder pvs to add metadata
	pvs, err := getPVsByCinderID(clientset)
	if err != nil {
		return err
	}

	// get all volumes from openstack
	mc := newOpenStackMetric("volume", "list")
	pages, err := volumes.List(client, volumes.ListOpts{}).AllPages()
	if mc.Observe(err) != nil {
		// only warn, maybe the next list will work.
		klog.Warningf("Unable to list volumes: %v", err)
		return err
	}
	volumesList, err := volumes.ExtractVolumes(pages)
	if err != nil {
		return err
	}

	// get quotas form openstack
	mc = newOpenStackMetric("volume_quotasets_usage", "get")
	quotas, err := quotasets.GetUsage(client, tenantID).Extract()
	if mc.Observe(err) != nil {
		// only warn, maybe the next get will work.
		klog.Warningf("Unable to get volume quotas: %v", err)
		return err
	}

	// second step: reset the old metrics
	// cinderQuotaVolumes and CinderQuotaVolumesGigabytes are not dynamic and do not need to be reset
	cinderVolumeCreated.Reset()
	cinderVolumeUpdatedAt.Reset()
	cinderVolumeStatus.Reset()
	cinderVolumeSize.Reset()
	cinderVolumeAttachedAt.Reset()

	// third step: publish the metrics
	publishVolumes(volumesList, pvs)

	publishCinderQuotas(quotas)
	return nil
}

// publishCinderQuotas publishes all cinder related quotas
func publishCinderQuotas(q quotasets.QuotaUsageSet) {
	cinderQuotaVolumes.WithLabelValues("in-use").Set(float64(q.Volumes.InUse))
	cinderQuotaVolumes.WithLabelValues("reserved").Set(float64(q.Volumes.Reserved))
	cinderQuotaVolumes.WithLabelValues("limit").Set(float64(q.Volumes.Limit))
	cinderQuotaVolumes.WithLabelValues("allocated").Set(float64(q.Volumes.Allocated))

	cinderQuotaVolumesGigabyte.WithLabelValues("in-use").Set(float64(q.Gigabytes.InUse))
	cinderQuotaVolumesGigabyte.WithLabelValues("reserved").Set(float64(q.Gigabytes.Reserved))
	cinderQuotaVolumesGigabyte.WithLabelValues("limit").Set(float64(q.Gigabytes.Limit))
	cinderQuotaVolumesGigabyte.WithLabelValues("allocated").Set(float64(q.Gigabytes.Allocated))
}

// publishVolumes iterates over a page, the result of a list request
func publishVolumes(vList []volumes.Volume, pvs map[string]corev1.PersistentVolume) {
	for _, v := range vList {
		if pv, ok := pvs[v.ID]; ok {
			publishVolumeMetrics(v, &pv)
		} else {
			publishVolumeMetrics(v, nil)
		}
	}
}

// publishVolumeMetrics extracts data from a volume and exposes the metrics via prometheus
func publishVolumeMetrics(v volumes.Volume, pv *corev1.PersistentVolume) {
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
