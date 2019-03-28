package kubeutils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  "k8s.io/client-go/kubernetes"
  "k8s.io/klog"
)

// GetClusterNodes returns kubernetes node objects for all nodes
func GetClusterNodes(*rest.Config) (nodes kubernetes.node[] err error) {
}
