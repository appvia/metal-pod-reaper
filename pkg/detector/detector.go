// Package detector detects if nodes are not contactable
// Both the source ip and the destination IP are reported
package detector

import (
	"time"

	"github.com/appvia/metal-pod-reaper/pkg/kubeutils"
	pinger "github.com/sparrc/go-ping"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

const (
	pingCount            = 5
	pingTimeout          = time.Second * pingCount
	detectorCMNamePrefix = "metal-pod-reaper"
	detectorCMPrefix     = "UnReachableIp"
	detectorCMSuffix     = "NodeName"
	detectorCMLabelName  = "creator"
)

// Detector provides data for detector methods
type Detector struct {
	c         chan error
	client    clientset.Interface
	dryRun    bool
	hostIP    string
	namespace string
}

// Create a struct for reporting on async Pinging...
type nodeDown struct {
	Err        error
	NetNode    kubeutils.NetNode
	IsNodeDown bool
}

// New creates a default detector
func New(dryRun bool, namespace, hostIP string) *Detector {
	d := &Detector{
		c:         make(chan error),
		dryRun:    dryRun,
		hostIP:    hostIP,
		namespace: namespace,
	}
	return d
}

// RunAsync will start the detector and return a channel for errors
func (d *Detector) RunAsync() chan (error) {
	go func() {
		defer close(d.c)
		if err := d.Run(); err != nil {
			// Return the error to the calling thread
			d.c <- err
		}
	}()
	return d.c
}

// Run starts the detector thread (blocking).
func (d *Detector) Run() error {
	cfg, err := kubeutils.BuildConfig()
	if err != nil {
		return err
	}
	d.client = clientset.NewForConfigOrDie(cfg)
	klog.Info("node down detector started")
	for {
		// Don't thrash here..
		time.Sleep(5 * time.Second)

		klog.V(5).Info("getting unready nodes")
		unreadyNodes, err := kubeutils.GetUnreadyNodes(d.client)
		if err != nil {
			klog.Errorf("error getting unschedulable nodes: %s", err)
			// No point digging, lets backoff
			time.Sleep(10 * time.Second)
			continue
		}
		if len(unreadyNodes.Items) < 1 {
			klog.V(3).Info("node down detector - all nodes ready")
			time.Sleep(10 * time.Second)
			continue
		}
		klog.Info("unready nodes detected")
		// For all the unready check which ones are checkable (have pingable address...)
		checkableNodes := make(map[string]nodeDown)
		for _, node := range unreadyNodes.Items {
			// Only check thos nodes with ip's
			ip, err := kubeutils.GetNodeInternalIP(&node)
			if err != nil {
				klog.Errorf("will not check node %s as problem getting internal ip: %s", node.Name, err)
			} else {
				checkableNodes[node.Name] = nodeDown{
					NetNode: kubeutils.NetNode{
						Node: &node,
						IP:   ip,
					},
				}
			}
		}
		// Create a buffered channel for all the checks
		results := make(chan nodeDown, len(checkableNodes))
		for _, node := range checkableNodes {
			// Do the checks concurrently:
			go func(nodeName string) {
				// Get the node details
				result := checkableNodes[nodeName]
				// do the check for this node
				klog.V(4).Infof("about to check node %s with ip %s", nodeName, result.NetNode.IP)
				nodeDown, err := isNodeDown(result.NetNode.IP)
				// record the results
				result.Err = err
				result.IsNodeDown = nodeDown
				if result.Err != nil {
					klog.V(4).Infof("error checking node %s, %s", result.NetNode.IP, result.Err)
				} else {
					if nodeDown {
						klog.V(4).Infof("node %s is unreachable, repeat NOT reachable", result.NetNode.IP)
					} else {
						klog.V(4).Infof("node %s is reachable", result.NetNode.IP)
					}
				}
				// Put the result on the channel (signal that the result is in)...
				results <- result
			}(node.NetNode.Node.Name)
		}
		var unReachableNodes []kubeutils.NetNode
		// Now wait till the results are in for all nodes:
		for nodeIndex := 1; nodeIndex <= len(checkableNodes); nodeIndex++ {
			klog.V(4).Infof("waiting for node result %d of %d", nodeIndex, len(checkableNodes))
			nodeResult := <-results
			klog.V(4).Infof("got node result %d of %d", nodeIndex, len(checkableNodes))
			if nodeResult.Err != nil {
				klog.Errorf("problem reporting on node ip %s: %s", nodeResult.NetNode.IP, nodeResult.Err)
			} else {
				if nodeResult.IsNodeDown {
					klog.V(1).Infof("unreachable node detetcted %s", nodeResult.NetNode.IP)
					unReachableNodes = append(unReachableNodes, nodeResult.NetNode)
				} else {
					klog.V(2).Infof("unready node still reachable %s", nodeResult.NetNode.IP)
				}
			}
			klog.V(4).Infof("completed processing node result %d of %d", nodeIndex, len(checkableNodes))
		}
		klog.V(4).Infof("we have reported on %d unreachable nodes", len(unReachableNodes))
		if len(unReachableNodes) > 0 {
			// Report on all failed nodes together:
			if err := kubeutils.ReportUnreachableIPs(d.client, unReachableNodes, d.hostIP, d.namespace); err != nil {
				klog.Errorf("problem reporting unreachable nodes: %s", err)
			}
			klog.V(2).Info("completed any reported on nodes down...")
		}
	}
}

func isNodeDown(ip string) (bool, error) {
	pinger, err := pinger.NewPinger(ip)
	if err != nil {
		return false, err
	}
	pinger.Timeout = pingTimeout
	pinger.Count = pingCount
	pinger.SetPrivileged(true)
	pinger.Run()
	if pinger.Statistics().PacketLoss == 100 {
		// This is a dead node from here - indicate this to the cluster...
		return true, nil
	}
	return false, nil
}
