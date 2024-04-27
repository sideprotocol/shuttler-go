package cmd

import (
	"github.com/sideprotocol/shuttler/app"
	"github.com/sideprotocol/shuttler/relayer"
	"github.com/spf13/cobra"
)

// NewVersionCommand returns a CLI command to interactively print the application binary version information.
func NewStartCommand(app *app.State) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the relayer",
		RunE: func(cmd *cobra.Command, _ []string) error {
			relayer.Start(app)
			return nil
		},
	}

	return cmd
}
