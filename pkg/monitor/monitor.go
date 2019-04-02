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

type MonitorReaper struct {
	c         chan error
	DryRun    bool
	Namespace string
	id        string
	Reap      bool
}

func NewMonitorReaper(reap, dryRun bool, namespace, id string) *MonitorReaper {
	m := &MonitorReaper{
		c:      make(chan error),
		Reap:   reap,
		DryRun: dryRun,
	}
	return m
}

// RunAsync starts the monitor thread
// - uses a channel for error handling
func (m *MonitorReaper) RunAsync() chan error {
	go func() {
		defer close(m.c)
		if err := m.runLeaderElect(); err != nil {
			// Return the error to the calling thread
			m.c <- err
		}
	}()
	return m.c
}

// runMonitorLoop is the core logic for the master component
// - called from the runLeaderElect - WHEN master
// - will return an error if it stops!
// - should only be run once in a cluster
func (m *MonitorReaper) runMonitorLoop() error {
	// Get all nodes in cluster
	cfg, err := kubeutils.BuildConfig()
	if err != nil {
		return err
	}
	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		return err
	}
	nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("can't list nodes: %s", err)
	}
	// nodesApi
	for {
		/*
		 1. Detect Quorum
		 2. Initiate Reap / Report
		*/

		if m.Reap {
			if err := reaper.Run("node", client, true); err != nil {
				log.Printf("format")
			}
		}
	}
}

// RunLeadderElect blocking - should never return (unless unrecoverable error)
// - Based on Kubernetes master locking example
// - see: https://github.com/kubernetes/client-go/blob/master/examples/leader-election/main.go
func (m *MonitorReaper) runLeaderElect() error {
	const leaseLockName = "metal-pod-reaper"

	cfg, err := kubeutils.BuildConfig()
	if err != nil {
		return err
	}
	client := clientset.NewForConfigOrDie(cfg)

	// we use the Lease lock type since edits to Leases are less common
	// and fewer objects in the cluster watch "all Leases".
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseLockName,
			Namespace: m.Namespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: m.id,
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
				klog.Infof("%s: leading", m.id)
				if err := m.runMonitorLoop(); err != nil {
					// Not sure how to signal death?
					log.Fatalf("unexpected exit of monitor thread")
				}
			},
			OnStoppedLeading: func() {
				// we can do cleanup here, or after the RunOrDie method
				// returns
				klog.Infof("%s: lost", m.id)
			},
			OnNewLeader: func(identity string) {
				// we're notified when new leader elected
				if identity == m.id {
					// I just got the lock
					return
				}
				klog.Infof("new leader elected: %v", identity)
			},
		},
	})

	// because the context is closed, the client should report errors
	_, err = client.CoordinationV1().Leases(m.Namespace).Get(leaseLockName, metav1.GetOptions{})
	if err == nil || !strings.Contains(err.Error(), "the leader is shutting down") {
		log.Fatalf("%s: expected to get an error when trying to make a client call: %v", m.id, err)
	}

	// we no longer hold the lease, so perform any cleanup and then
	// exit
	log.Printf("%s: done", m.id)
	return errors.New("err what, how?")
}
