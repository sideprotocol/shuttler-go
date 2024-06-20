package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
)

// NewVersionCommand returns a CLI command to interactively print the application binary version information.
func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the application binary version information",
		RunE: func(cmd *cobra.Command, _ []string) error {

			fmt.Printf("Version: %s\n", version)

			if long, err := cmd.Flags().GetBool("long"); err == nil && long {

				deps, ok := debug.ReadBuildInfo()
				if !ok {
					return fmt.Errorf("unable to read deps")
				}

				fmt.Println("BuildDeps:")

				for _, dep := range deps.Deps {
					fmt.Printf("\t%s => %s\n", dep.Path, dep.Version)
				}
			}

			return nil
		},
	}

	cmd.PersistentFlags().Bool("long", false, "Print long version information")

	return cmd
}
