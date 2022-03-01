package context

import (
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

type Context struct {
	Pods      []v1.Pod
	Services  []v1.Service
	Ingresses []networkingv1.Ingress
}
