package cmd

import (
	"bufio"
	"os"
	"strings"

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

			network, err := cmd.Flags().GetString("network")
			if err != nil {
				return err
			}

			generate, err := cmd.Flags().GetBool("generate")
			if err != nil {
				return err
			}
			mnemonic := ""
			if !generate {
				println("Please input your mnemonic: ")
				reader := bufio.NewReader(os.Stdin)
				mnemonic, err = reader.ReadString('\n')
				if err != nil {
					return err
				}
			}

			cb := app.NewConfigBuilder(home)
			cb.InitConfig(strings.TrimSpace(mnemonic), network)
			println("\nConfiguration file created at: ", cb.ConfigFilePath())

			return nil
		},
	}

	cmd.PersistentFlags().Bool("generate", false, "Generate a new mnemonic for the keyring instead of recovering an existing one")
	cmd.PersistentFlags().String("network", "mainnet", "The network to use (mainnet, testnet, regtest, simnet)")

	return cmd
}
