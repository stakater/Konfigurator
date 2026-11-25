package volumes

import (
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GetVolumesFunc func(interface{}) []corev1.Volume
type VolumeFunc func(int, corev1.Volume)

// GetFromObject returns the pod-template volumes of a Deployment, DaemonSet or
// StatefulSet. It dispatches on the Go type, not TypeMeta: controller-runtime's
// typed client clears TypeMeta after a Get (H.7). Unsupported types yield nil.
func GetFromObject(target metav1.Object) []corev1.Volume {
	switch target.(type) {
	case *v1.Deployment:
		return GetDeploymentVolumes(target)
	case *v1.DaemonSet:
		return GetDaemonSetVolumes(target)
	case *v1.StatefulSet:
		return GetStatefulSetVolumes(target)
	}
	return nil
}

func GetDeploymentVolumes(target interface{}) []corev1.Volume {
	return target.(*v1.Deployment).Spec.Template.Spec.Volumes
}

func GetDaemonSetVolumes(target interface{}) []corev1.Volume {
	return target.(*v1.DaemonSet).Spec.Template.Spec.Volumes
}

func GetStatefulSetVolumes(target interface{}) []corev1.Volume {
	return target.(*v1.StatefulSet).Spec.Template.Spec.Volumes
}

func ForEach(volumes []corev1.Volume, volumeFunc VolumeFunc) {
	for index, volume := range volumes {
		volumeFunc(index, volume)
	}
}
