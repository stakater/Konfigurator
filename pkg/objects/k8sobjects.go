package objects

import (
	quotav1 "github.com/openshift/api/quota/v1"
	userv1 "github.com/openshift/api/user/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Secret returns a new secret specified with the name and namespace
func Secret(name string, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

// EmptySecret returns a new empty secret
func EmptySecret() *corev1.Secret {
	return &corev1.Secret{}
}

// Namespace returns a new namespace specified with the name
func Namespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// EmptyNamespace returns a new namespace
func EmptyNamespace() *corev1.Namespace {
	return &corev1.Namespace{}
}

// Group returns a new group specified with the name
func Group(name string) *userv1.Group {
	return &userv1.Group{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// EmptyGroup returns a new empty group
func EmptyGroup() *userv1.Group {
	return &userv1.Group{}
}

// ResourceQuota returns a new ResourceQuota specified with the name and namespace
func ResourceQuota(name string, namespace string) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// ClusterResourceQuota returns a new ClusterResourceQuota specified with the name
func ClusterResourceQuota(name string) *quotav1.ClusterResourceQuota {
	return &quotav1.ClusterResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// RoleBinding returns a new RoleBinding specified with the name and namespace
func RoleBinding(name string, namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// EmptyResourceQuota returns a new empty group
func EmptyResourceQuota() *corev1.ResourceQuota {
	return &corev1.ResourceQuota{}
}
