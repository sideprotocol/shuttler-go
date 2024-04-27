package cmd

import (
	"github.com/pelletier/go-toml/v2"
	"github.com/sideprotocol/shuttler/app"
	"github.com/spf13/cobra"
)

// NewVersionCommand returns a CLI command to interactively print the application binary version information.
func NewInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the configuration file",
		RunE: func(cmd *cobra.Command, _ []string) error {

			home, err := cmd.Flags().GetString("home")
			if err != nil {
				return err
			}

			cb := app.NewConfigBuilder(home)
			cfg := cb.InitConfig()
			println("Configuration file created at: ", cb.ConfigFilePath())

			out, err := toml.Marshal(*cfg)
			if err != nil {
				return err
			}
			println(string(out))
			return nil
		},
	}

	return cmd
}
