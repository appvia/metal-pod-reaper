package cmd

import (
	"flag"
	"fmt"
	"os"
	"strconv"

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

	flag.BoolVar(&dryRun, "dry-run", true, "only report on potential changes (env - DRY_RUN)")
	flag.BoolVar(&reap, "no-reap", true, "do not run the reap facility")
	flag.StringVar(&namespace, "namespace", "", "namespace for the master leaselock object (env - NAMESPACE)")
	flag.StringVar(&hostIP, "host-ip", "", "specify the host ip (env - HOST_IP)")
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
	dryRunStr := os.Getenv("DRY_RUN")
	if len(dryRunStr) > 0 {
		if b, err := strconv.ParseBool(dryRunStr); err != nil {
			klog.Fatalf("Expecting bool in DRY_RUN not %s", dryRunStr)
		} else {
			dryRun = b
		}
	}
	if err := mpodr.Run(reap, dryRun, namespace, hostIP); err != nil {
		klog.Fatalf("Metal POD reaper failed:%s", err)
	}
}
