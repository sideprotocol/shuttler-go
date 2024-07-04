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

			keyType, err := cmd.Flags().GetString("key-type")
			if err != nil {
				return err
			}

			generate, err := cmd.Flags().GetBool("generate")
			if err != nil {
				return err
			}
			mnemonic := ""
			if !generate {
				println("Please input relayer mnemonic: ")
				reader := bufio.NewReader(os.Stdin)
				mnemonic, err = reader.ReadString('\n')
				if err != nil {
					return err
				}
			}

			localVaultEnabled, err := cmd.Flags().GetBool("local-vault")
			if err != nil {
				return err
			}

			mVaults := make([]string, 0)

			if localVaultEnabled {
				println("Please input vault mnemonics, end with Ctrl+D")
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					mVaults = append(mVaults, strings.TrimSpace(scanner.Text()))
				}
			}

			cb := app.NewConfigBuilder(home)
			cb.InitConfig(strings.TrimSpace(mnemonic), mVaults, network, strings.TrimSpace(keyType))
			println("\nConfiguration file created at: ", cb.ConfigFilePath())

			return nil
		},
	}

	cmd.PersistentFlags().Bool("generate", false, "Generate a new mnemonic for the keyring instead of recovering an existing one")
	cmd.PersistentFlags().String("network", "mainnet", "The network to use (mainnet, testnet, regtest, simnet)")
	cmd.PersistentFlags().String("key-type", "segwit", "The key type (segwit, taproot)")
	cmd.PersistentFlags().Bool("local-vault", false, "Enable local vault for testing")

	return cmd
}
