package cmd

import (
	"flag"

	"k8s.io/klog"
)

func Execute() {

	klog.InitFlags(nil)

	var leaseLockName string
	var leaseLockNamespace string
	var id string

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&id, "id", "", "the holder identity name")
	flag.StringVar(&leaseLockName, "lease-lock-name", "example", "the lease lock resource name")
	flag.StringVar(&leaseLockNamespace, "lease-lock-namespace", "", "the lease lock resource namespace")
	flag.Parse()

}
