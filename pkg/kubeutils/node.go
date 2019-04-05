package kubeutils

import (
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

const (
	nodeConfigMapNamePrefix      = "unreachable-nodes-from.mprodr.x.x.x.x"
	configMapKeyLastChecked      = "lastChecked"
	configMapKeyUnreachableNodes = "unreachableNodesCSV"
	configMapKeyCheckedBy        = "checkedByIP"
	configMapLabelName           = "unreachable-nodes-from.mprodr"
	configMapLabelValue          = "true"
	configMapValidFor            = 60 * time.Second
)

// NetNode provides details of which node can't be contacted
type NetNode struct {
	IP   string
	Node *v1.Node
}

// GetUnreadyNodes returns all the nodes that could be down
func GetUnreadyNodes(c clientset.Interface) (*v1.NodeList, error) {
	// First get all the nodes
	nodes, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		// Maybe we should be retrying...?
		return nil, fmt.Errorf("can't list nodes: %s", err)
	}

	unReadyNodes := make([]v1.Node, 0)
	for _, n := range nodes.Items {
		for _, c := range n.Status.Conditions {
			if (c.Type == v1.NodeReady) {
				klog.V(5).Infof("got node type %s for node %s (with status %s)", v1.NodeReady, n.Name, c.Status)
				if (c.Status != v1.ConditionTrue) {
					klog.V(5).Infof("NotReady Node found %s", n.Name)
					unReadyNodes = append(unReadyNodes, n)
				}
				continue // no need to inspect other node conditions
			}
		}
	}
	return &v1.NodeList{Items: unReadyNodes}, nil
}

// GetNodeInternalIP returns the internal IP address of the node object
// Maybe we should error if there's more than a single IP unless
// opted in (as workloads could be commiting remote data)
func GetNodeInternalIP(node *v1.Node) (string, error) {
	host := ""
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			if address.Address != "" {
				host = address.Address
				break
			}
		}
	}
	if host == "" {
		return "", fmt.Errorf("Couldn't get the internal IP of host %s with addresses %v", node.Name, node.Status.Addresses)
	}
	return host, nil
}

// ReportUnreachableIPs records the ip addresses that can't be contacted
// - Used by the detector thread to report all node(s) that are unreachable (from a given source)
func ReportUnreachableIPs(c clientset.Interface, unreachableNodes []NetNode, reportingNodeIP string, namespace string) error {
	/*
		Create a unique configmap for the detector with shared label e.g.:

		kind: ConfgMap
		metadata:
			name: unreachable-nodes-from.mprodr.x.x.x.x
			labels:
			  source: mpodr-unreachable-results

		data:
			lastChecked: datetime
			unreachableNodes: name,name,name
			checkedBy: ip
	*/
	var unreachableNodeNames []string
	for _, un := range unreachableNodes {
		unreachableNodeNames = append(unreachableNodeNames, un.Node.Name)
	}
	cmName := getConfigMapName(reportingNodeIP)
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      cmName,
			Labels: map[string]string{
				configMapLabelName: configMapLabelValue,
			},
		},
		Data: map[string]string{
			configMapKeyLastChecked:      fmt.Sprintf("%s", time.Now().Format(time.RFC3339)),
			configMapKeyUnreachableNodes: strings.Join(unreachableNodeNames, ","),
			configMapKeyCheckedBy:        reportingNodeIP,
		},
	}

	// Discover if object exists and create / update as appropriate:
	var create bool
	_, err := c.CoreV1().ConfigMaps(namespace).Get(cmName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			create = false
		} else {
			return fmt.Errorf("error discovering if configmap %s exists: %s", cmName, err)
		}
	} else {
		create = true
	}
	if create {
		_, err = c.CoreV1().ConfigMaps(namespace).Create(cm)
	} else {
		_, err = c.CoreV1().ConfigMaps(namespace).Update(cm)
	}
	return err
}

// GetUnreachableNodes get nodes that are REPORTED as unreachanble by the function above
// - used from the monitor thread to provide a consensus of node Unreachability
func GetUnreachableNodes(c clientset.Interface, namespace string) ([]*v1.Node, error) {
	/*
		1. Select configmaps by label
		2. Verify the reported datetime is within threasholds
		3. Get a list of Unreachable nodes that have a quorum of results
	*/
	var unreachableNodes []*v1.Node

	// get ConfigMaps "reporting node Unreachable"
	cmOptions := metav1.ListOptions{
		LabelSelector: configMapLabelName + "=" + configMapLabelValue,
	}
	cmList, err := c.CoreV1().ConfigMaps(namespace).List(cmOptions)
	if err != nil {
		return nil, fmt.Errorf("error getting configmaps: %s", err)
	}
	allNodes, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		// Maybe we should be retrying...?
		return nil, fmt.Errorf("can't list nodes: %s", err)
	}
	unreadyNodes, err := GetUnreadyNodes(c)
	if err != nil {
		// Maybe we should be retrying...?
		return nil, fmt.Errorf("problem getting list of UnReady nodes %s", err)
	}
	reportingQuorum := len(allNodes.Items) - len(unreadyNodes.Items)
	unReachableReportCountsByHost := make(map[string]int)
	for _, cm := range cmList.Items {
		// check the cm checkTime is valid:
		reportTimeStr := cm.Data[configMapKeyLastChecked]
		reportTime, err := time.Parse(time.RFC3339, reportTimeStr)
		if err != nil {
			klog.Errorf("cannot parse datetime value %s in configmap %s error=%s", reportTimeStr, cm.Name, err)
			// discount this report
			continue
		}
		if reportTime.Before(time.Now().Add(configMapValidFor)) {
			// REAP contender
			for _, nodeName := range strings.Split((cm.Data[configMapKeyUnreachableNodes]), ",") {
				unReachableReportCountsByHost[nodeName]++
			}
		}
	}
	// Work out if all nodes that have reported agree (above the quorum threashold)
	for _, node := range unreadyNodes.Items {
		if _, ok := unReachableReportCountsByHost[node.Name]; ok {
			if unReachableReportCountsByHost[node.Name] >= reportingQuorum {
				unreachableNodes = append(unreachableNodes, &node)
			}
		}
	}
	return unreachableNodes, nil
}

func getConfigMapName(sourceIP string) string {
	return fmt.Sprintf("%s.%s", nodeConfigMapNamePrefix, sourceIP)
}
