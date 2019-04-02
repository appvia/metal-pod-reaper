// Will detect if any node is uncontactable!
package detector

func RunAsync(dryRun bool) chan (error) {
	c := make(chan error)

	go func() {
		defer close(c)
		if err := Run(dryRun); err != nil {
			// Return the error to the calling thread
			c <- err
		}
	}()
	return c
}

// Run starts the detector thread.
func Run(dryRun bool) error {

	/*
		cl, err := kubeutils.BuildConfig()

		if err != nil {
			return err
		}
	*/
	// First get all node objects from the cluster:
	return nil
}
