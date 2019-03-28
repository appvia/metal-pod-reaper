package cmd

import (
	"fmt"

	"github.com/appvia/metal-pod-reaper/pkg/version"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of mpodr",
	Long:  `All software has versions. This is mpodr's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%+v\n", version.Get())
	},
}
