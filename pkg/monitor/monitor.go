/*
	Package to detect a Quorum and and to report or invoke the reaper
*/
package monitor

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/appvia/metal-pod-reaper/pkg/kubeutils"
	"github.com/appvia/metal-pod-reaper/pkg/reaper"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

const (
	leaseDuration    = 15 * time.Second
	renewDeadline    = 10 * time.Second
	retryPeriod      = 5 * time.Second
	pausePollingSecs = 5 * time.Second
)

// Monitor data for Monitor methods
type Monitor struct {
	c         chan error
	dryRun    bool
	namespace string
	hostIP    string
	reap      bool
}

// New creates a default monitor / reaper
func New(reap, dryRun bool, namespace, hostIP string) *Monitor {
	m := &Monitor{
		c:         make(chan error),
		dryRun:    dryRun,
		hostIP:    hostIP,
		namespace: namespace,
		reap:      reap,
	}
	return m
}

// RunAsync starts the monitor thread
// - uses a channel for error handling
func (m *Monitor) RunAsync() chan error {
	klog.V(5).Info("starting leader elect bit")
	go func() {
		defer close(m.c)
		if err := m.runLeaderElect(); err != nil {
			// Return the error to the calling thread
			m.c <- err
		}
	}()
	klog.V(5).Info("starting leader elect bit started")
	return m.c
}

// runMonitorLoop is the core logic for the master component
// - called from the runLeaderElect - WHEN master
// - will return an error if it stops!
// - should only be run once in a cluster
func (m *Monitor) runMonitorLoop() error {
	// Get all nodes in cluster
	cfg, err := kubeutils.BuildConfig()
	if err != nil {
		return err
	}
	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		return err
	}
	klog.Info("started master")
	var deadNodes []*v1.Node
	for {
		// Don't thrash here..
		klog.V(4).Info("little pause before work")
		time.Sleep(pausePollingSecs)

		// Get all the nodes - that have been reported as UnReachable...
		// reporting happens using configmaps in specified namespace
		deadNodes, err = kubeutils.GetUnreachableNodes(client, m.namespace)
		if err != nil {
			klog.Errorf("error getting nodes reported as unreachable: %s", err)
			// Try again
			continue
		}
		klog.V(3).Infof("got an unreachable node list (%d nodes)", len(deadNodes))

		// reap any nodes as required...
		if m.reap && len(deadNodes) > 0 {
			klog.V(4).Info("We are set to reap")
			for _, node := range deadNodes {
				if err := reaper.Reap(node, client, m.dryRun); err != nil {
					klog.Errorf("error reaping %s, %s", node.Name, err)
				}
			}
		}
	}
}

// RunLeadderElect blocking - should never return (unless unrecoverable error)
// - Based on Kubernetes master locking example
// - see: https://github.com/kubernetes/client-go/blob/master/examples/leader-election/main.go
func (m *Monitor) runLeaderElect() error {
	klog.Info("started master component (not master yet)")
	const leaseLockName = "metal-pod-reaper"

	cfg, err := kubeutils.BuildConfig()
	if err != nil {
		return err
	}
	client := clientset.NewForConfigOrDie(cfg)

	// wrap the callback function:
	ctxRunMonitorLoop := func(ctx context.Context) {
		if err := m.runMonitorLoop(); err != nil {
			// Not sure how to signal death?
			klog.Fatal("unexpected exit of monitor thread")
		}
	}

	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: client.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: leaseLockName})

	rlConfig := resourcelock.ResourceLockConfig{
		Identity:      m.hostIP,
		EventRecorder: recorder,
	}
	lock, err := resourcelock.New(
		resourcelock.ConfigMapsResourceLock,
		m.namespace,
		leaseLockName,
		client.CoreV1(),
		rlConfig,
	)

	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	leaderConfig := leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: leaseDuration,
		RenewDeadline: renewDeadline,
		RetryPeriod:   retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.V(2).Info("Became leader, starting")
				ctxRunMonitorLoop(ctx)
			},
			OnStoppedLeading: func() {
				klog.Fatal("Stopped leading")
			},
			OnNewLeader: func(identity string) {
				klog.V(3).Infof("Current leader: %s", identity)
			},
		},
	}

	leaderelection.RunOrDie(context.TODO(), leaderConfig)
	return errors.New("leader locking exited master")
}

func recordEvent(e string) {
	klog.V(2).Infof("event: %s", e)
}
