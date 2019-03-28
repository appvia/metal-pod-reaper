// Will detect if any node is uncontactable!
package detector

import (
	"github.com/appvia/metal-pod-reaper/pkg/kubeutils"
)

func RunAsync(dryRun bool) chan error {
	errorCh := make(chan error)

	go func() {
		defer close(c)
		if err := Run(dryRun); err != nil {
			// Return the error to the calling thread
			errorCh <- err
		}
	}()
	return errorCh
}

// Run starts the detector thread.
func Run(dryRun bool) {

	cl, err := kubeutils.BuildConfig()

	if err != nil {
		return err
	}

	// First get all node objects from the cluster:

}
