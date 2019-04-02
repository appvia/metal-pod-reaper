package cmd

import (
	"flag"
	"fmt"

	"github.com/appvia/metal-pod-reaper/pkg/mpodr"
	"github.com/appvia/metal-pod-reaper/pkg/version"
	"k8s.io/klog"
)

func Execute() {

	klog.InitFlags(nil)

	var dryRun bool
	var reap bool
	var version bool
	var namespace string
	var id string

	flag.BoolVar(&dryRun, "dry-run", true, "only report on potential changes")
	flag.BoolVar(&reap, "reap", true, "do not run the reap facility")
	flag.StringVar(&namespace, "namespace", "", "namespace for the master leaselock object")
	flag.StringVar(&id, "id", "", "specify the id (defaults to hostname)")
	flag.BoolVar(&version, "version", false, "display the version")
	flag.Parse()

	if version {
		version()
		os.Exit(0)
	}
	if namespace == "" {
		namespace = os.Getenv("MPODR_NAMESPACE")
	}
	if id == "" {
		id = os.Getenv("MPODR_ID")
		if id == "" {
			id = os.Getenv("HOSTNAME")
		}
	}
	if err := mpodr.Run(reap, dryRun, namespace, id); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func version(cmd *cobra.Command, args []string) {
		fmt.Printf("%+v\n", version.Get())
	},
}
