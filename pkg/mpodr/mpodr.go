// Package mpodr detects if a node is uncontactable!
package mpodr

import (
	"errors"

	"github.com/appvia/metal-pod-reaper/pkg/detector"
	"github.com/appvia/metal-pod-reaper/pkg/monitor"
)

// Run starts the mpodr (metal pod reaper) threads
func Run(reap, dryRun bool, namespace, hostIP string) error {

	// Start a background thread for running the Monitor
	//  this will detect a quorum and invokes the reaper
	// should NOT return
	m := monitor.New(reap, dryRun, namespace, hostIP)
	mCh := m.RunAsync()

	// Start a background to run the detector
	// should NOT return
	d := detector.New(dryRun, namespace, hostIP)
	dCh := d.RunAsync()

	c := make(chan error)
	// Merge any errors into a single channels
	go func() {
		defer close(c)
		for mCh != nil || dCh != nil {
			select {
			case v, ok := <-dCh:
				if !ok {
					dCh = nil
					continue
				}
				c <- v
			case v, ok := <-mCh:
				if !ok {
					mCh = nil
					continue
				}
				c <- v
			}
		}
	}()

	// Block and wait for either channel to return an error
	if err := <-c; err != nil {
		// Either chennel should only exit with an error - time to go!
		return err
	}
	// Should never get here!
	return errors.New("Unexpected return - All threads have closed thier channels with no errors!")
}
