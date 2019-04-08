// Package reaper will detect if a node is uncontactable!
package reaper

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

// Reap starts deleteing pods from an UnReady node
// - Should ONLY delete STS and Deployment Pods
// - Does NOT need to cordon (as the node is UnReady)
func Reap(node *v1.Node, cl *kubernetes.Clientset, dryRun bool) error {

	// Get the pods on this node
	pods, err := cl.CoreV1().Pods("").List(metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + node.Name,
	})
	if err != nil {
		return fmt.Errorf("error reaping: %s", node.Name)
	}
	klog.V(4).Infof("set to reap %d pods from %s", len(pods.Items), node.Name)

	var dryRunValue []string
	if dryRun {
		dryRunValue = []string{"All"}
	}
	// Define a 0 grace period (equiv to delete now)
	var gracePeriod int64
	// Equiv to force?
	orphanDependents := true
	for _, pod := range pods.Items {
		klog.Infof("reaping %s from %s (dry-run=%t)", pod.Name, node.Name, dryRun)
		err := cl.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{
			DryRun:             dryRunValue,
			OrphanDependents:   &orphanDependents,
			GracePeriodSeconds: &gracePeriod,
		})
		if err != nil {
			klog.Errorf("error reaping pod %s from %s:%s", pod.Name, node.Name, err)
		}
		klog.Infof("pod %s deleted from %s (dry-run=%t)", pod.Name, node.Name, dryRun)
	}
	return nil
}
