// SPDX-License-Identifier: MIT

package metrics

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

const (
	cinderCSIDriver = "cinder.csi.openstack.org"
)

func getPVsByCinderID(clientset *kubernetes.Clientset) (map[string]corev1.PersistentVolume, error) {
	pvsList, err := clientset.CoreV1().PersistentVolumes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to list pvs: %s", err)
	}

	pvs := map[string]corev1.PersistentVolume{}
	for _, pv := range pvsList.Items {
		if pv.Spec.Cinder != nil {
			pvs[pv.Spec.Cinder.VolumeID] = pv
		} else if pv.Spec.CSI != nil {
			if pv.Spec.CSI.Driver == cinderCSIDriver {
				pvs[pv.Spec.CSI.VolumeHandle] = pv
			} else {
				klog.V(8).Infof("ignoring pv %s: unimplemented csi-driver", pv.GetName())
			}
		} else {
			klog.V(8).Infof("ignoring pv %s: unimplemented volume plugin", pv.GetName())
		}
	}

	return pvs, nil
}

// extractK8sMetadata tries to extract the following data from a pv:
// "pvc_name", "pvc_namespace", "pv_name", "storage_class", "reclaim_policy", "fs_type"
func extractK8sMetadata(pv *corev1.PersistentVolume) []string {
	if pv == nil {
		return []string{"", "", "", "", "", ""}
	}

	var fsType string
	if pv.Spec.Cinder != nil {
		fsType = pv.Spec.Cinder.FSType
	} else if pv.Spec.CSI != nil {
		fsType = pv.Spec.CSI.FSType
	}

	var claimName, claimNamespace string
	if pv.Spec.ClaimRef != nil {
		claimName = pv.Spec.ClaimRef.Name
		claimNamespace = pv.Spec.ClaimRef.Namespace
	}

	return []string{
		claimName,
		claimNamespace,
		pv.GetName(),
		pv.Spec.StorageClassName,
		string(pv.Spec.PersistentVolumeReclaimPolicy),
		fsType,
	}
}
