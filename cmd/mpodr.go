package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/appvia/metal-pod-reaper/pkg/mpodr"
	"github.com/appvia/metal-pod-reaper/pkg/version"
	"k8s.io/klog"
)

// Execute provides the entrypoint from main
func Execute() {

	klog.InitFlags(nil)

	var dryRun bool
	var reap bool
	var ver bool
	var namespace string
	var hostIP string

	flag.BoolVar(&dryRun, "dry-run", true, "only report on potential changes")
	flag.BoolVar(&reap, "no-reap", true, "do not run the reap facility")
	flag.StringVar(&namespace, "namespace", "", "namespace for the master leaselock object")
	flag.StringVar(&hostIP, "host-ip", "", "specify the host ip")
	flag.BoolVar(&ver, "version", false, "display the version")
	flag.Parse()

	if ver {
		fmt.Printf("%+v\n", version.Get())
		os.Exit(0)
	}
	if namespace == "" {
		namespace = os.Getenv("NAMESPACE")
		if namespace == "" {
			// TODO get from API or /run/secrets/kubernetes.io/serviceaccount/namespace
			klog.Fatal("Expecting NAMESPACE to be set")
		}
	}
	if hostIP == "" {
		hostIP = os.Getenv("HOST_IP")
		if hostIP == "" {
			klog.Fatal("Expecting HOST_IP to be set")
		}
	}
	if err := mpodr.Run(reap, dryRun, namespace, hostIP); err != nil {
		klog.Fatalf("Metal POD reaper failed:%s", err)
	}
}
