// Will detect if a node is uncontactable!
package reaper

import (
	"errors"

	"k8s.io/client-go/kubernetes"
)

// Run starts the reaper thread.
func Run(node string, cl *kubernetes.Clientset, dryRun bool) error {

	return errors.New("Notimplimented")

	/*
	  Should maybe cordon a node and reap pods here!
	*/
}
