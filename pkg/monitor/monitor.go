/*
	Package to detect a Quorum and and to report or invoke the reaper
*/
package monitor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/appvia/metal-pod-reaper/pkg/kubeutils"
	"github.com/appvia/metal-pod-reaper/pkg/reaper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/transport"
	"k8s.io/klog"
)

// RunAsync starts the monitor thread
// - uses a channel for error handling
func RunAsync(reap, dryRun bool) chan (error) {
	errorCh := make(chan error)

	go func() {
		defer close(c)
		if err := Run(reap, dryRun); err != nil {
			// Return the error to the calling thread
			errorCh <- err
		}
	}()
	return errorCh
}

// runMonitorLoop is the core logic for the master component
// - called from the reunLeaderElect - WHEN master
// - will return an error if it stops!
// - should only be run once in a cluster
func runMonitorLoop() error {
	// Get all nodes in cluster

	for {
		/*
		 1. Detect Quorum
		 2. Initiate Reap / Report
		*/

		if reap {
			if err := reaper.Run("node", true); err != nil {
				log.Printf("format")
			}
		}
	}
}

// RunLeadderElect blocking - should never return (unless unrecoverable error)
// - Based on Kubernetes master locking example
// - see: https://github.com/kubernetes/client-go/blob/master/examples/leader-election/main.go
func runLeaderElect(reap, dryRun bool, leaseLockNamespace, id string) error {

	const leaseLockName = "metal-pod-reaper"

	if cfg, err := kubeutils.BuildConfig(); err != nil {
		return err
	}
	client := clientset.NewForConfigOrDie(config)

	// we use the Lease lock type since edits to Leases are less common
	// and fewer objects in the cluster watch "all Leases".
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseLockName,
			Namespace: leaseLockNamespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	// use a Go context so we can tell the leaderelection code when we
	// want to step down
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// use a client that will stop allowing new requests once the context ends
	config.Wrap(transport.ContextCanceller(ctx, fmt.Errorf("the leader is shutting down")))

	// listen for interrupts or the Linux SIGTERM signal and cancel
	// our context, which the leader election code will observe and
	// step down
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		log.Printf("Received termination, signaling shutdown")
		cancel()
	}()

	// start the leader election code loop
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock: lock,
		// IMPORTANT: you MUST ensure that any code you have that
		// is protected by the lease must terminate **before**
		// you call cancel. Otherwise, you could have a background
		// loop still running and another process could
		// get elected before your background loop finished, violating
		// the stated goal of the lease.
		ReleaseOnCancel: true,
		LeaseDuration:   60 * time.Second,
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				// we're notified when we start - this is where you would
				// usually put your code
				klog.Infof("%s: leading", id)
				if err := RunMaster(); err != nil {
					// Not sure how to signal death?
					log.Fatalf("unexpected exit of monitor thread")
				}
			},
			OnStoppedLeading: func() {
				// we can do cleanup here, or after the RunOrDie method
				// returns
				klog.Infof("%s: lost", id)
			},
			OnNewLeader: func(identity string) {
				// we're notified when new leader elected
				if identity == id {
					// I just got the lock
					return
				}
				klog.Infof("new leader elected: %v", identity)
			},
		},
	})

	// because the context is closed, the client should report errors
	_, err = client.CoordinationV1().Leases(leaseLockNamespace).Get(leaseLockName, metav1.GetOptions{})
	if err == nil || !strings.Contains(err.Error(), "the leader is shutting down") {
		log.Fatalf("%s: expected to get an error when trying to make a client call: %v", id, err)
	}

	// we no longer hold the lease, so perform any cleanup and then
	// exit
	log.Printf("%s: done", id)
	return errors.New("err what, how?")
}
