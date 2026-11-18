package containers

import (
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GetContainersFunc func(interface{}) []corev1.Container
type ContainerFunc func(int, corev1.Container)

// GetFromObject returns the pod-template containers of a Deployment, DaemonSet or
// StatefulSet. It dispatches on the Go type, not TypeMeta: controller-runtime's
// typed client clears TypeMeta after a Get (H.7). Unsupported types yield nil.
func GetFromObject(target metav1.Object) []corev1.Container {
	switch target.(type) {
	case *v1.Deployment:
		return GetDeploymentContainers(target)
	case *v1.DaemonSet:
		return GetDaemonSetContainers(target)
	case *v1.StatefulSet:
		return GetStatefulSetContainers(target)
	}
	return nil
}

func GetDeploymentContainers(target interface{}) []corev1.Container {
	return target.(*v1.Deployment).Spec.Template.Spec.Containers
}

func GetDaemonSetContainers(target interface{}) []corev1.Container {
	return target.(*v1.DaemonSet).Spec.Template.Spec.Containers
}

func GetStatefulSetContainers(target interface{}) []corev1.Container {
	return target.(*v1.StatefulSet).Spec.Template.Spec.Containers
}

func ForEach(containers []corev1.Container, containerFunc ContainerFunc) {
	for index, container := range containers {
		containerFunc(index, container)
	}
}
