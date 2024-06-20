package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/sideprotocol/shuttler/app"
)

const (
// Default identifiers for dummy usage
// dcli = "defaultclientid"
// dcon = "defaultconnectionid"
)

// NewRootCmd returns the root command for relayer.
// If log is nil, a new zap.Logger is set on the app state
// based on the command line flags regarding logging.
func NewRootCmd(log *zap.Logger) *cobra.Command {
	// Use a local app state instance scoped to the new root command,
	// so that tests don't concurrently access the state.
	var a = app.NewAppState("")

	// RootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Use:   app.AppName,
		Short: "This application makes data relay between Bitcoin and SIDE chain easy!",
		Long:  strings.TrimSpace(`Shuttler is a tool for relaying data between Bitcoin and SIDE chain.`),
	}

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {

		// Skip loading configuration for `init` and `version` commands.
		if cmd.Name() == "init" || cmd.Name() == "version" {
			return nil
		}

		// Inside persistent pre-run because this takes effect after flags are parsed.
		// reads `homeDir/config/config.yaml` into `a.Config`
		if err := a.Init(); err != nil {
			return err
		}

		return nil
	}

	rootCmd.PersistentPostRun = func(cmd *cobra.Command, _ []string) {
		// Force syncing the logs before exit, if anything is buffered.
		if a.Log != nil {
			_ = a.Log.Sync()
		}
	}

	// Register --home flag
	rootCmd.PersistentFlags().StringVar(&a.HomePath, "home", app.DefaultHome, "set home directory")
	if err := a.Viper.BindPFlag("home", rootCmd.PersistentFlags().Lookup("home")); err != nil {
		panic(err)
	}

	// Register --debug flag
	rootCmd.PersistentFlags().BoolVarP(&a.Debug, "debug", "d", false, "debug output")
	if err := a.Viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug")); err != nil {
		panic(err)
	}

	rootCmd.PersistentFlags().String("log-format", "auto", "log output format (auto, logfmt, json, or console)")
	if err := a.Viper.BindPFlag("log-format", rootCmd.PersistentFlags().Lookup("log-format")); err != nil {
		panic(err)
	}

	// Register --log-level flag
	rootCmd.PersistentFlags().String("log-level", "", "log level format (info, debug, warn, error, panic or fatal)")
	if err := a.Viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level")); err != nil {
		panic(err)
	}

	// Register subcommands
	rootCmd.AddCommand(
		keys.Commands(app.DefaultHome),
		NewInitCommand(),
		NewStartCommand(a),
		version.NewVersionCommand(),
	)

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.EnableCommandSorting = false

	rootCmd := NewRootCmd(nil)
	rootCmd.SilenceUsage = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt) // Using signal.Notify, instead of signal.NotifyContext, in order to see details of signal.
	go func() {
		// Wait for interrupt signal.
		sig := <-sigCh

		// Cancel context on root command.
		// If the invoked command respects this quickly, the main goroutine will quit right away.
		cancel()

		// Short delay before printing the received signal message.
		// This should result in cleaner output from non-interactive commands that stop quickly.
		time.Sleep(250 * time.Millisecond)
		fmt.Fprintf(os.Stderr, "Received signal %v. Attempting clean shutdown. Send interrupt again to force hard shutdown.\n", sig)

		// Dump all goroutines on panic, not just the current one.
		debug.SetTraceback("all")

		// Block waiting for a second interrupt or a timeout.
		// The main goroutine ought to finish before either case is reached.
		// But if a case is reached, panic so that we get a non-zero exit and a dump of remaining goroutines.
		select {
		case <-time.After(time.Minute):
			panic(errors.New("daemon did not shut down within one minute of interrupt"))
		case sig := <-sigCh:
			panic(fmt.Errorf("received signal %v; forcing quit", sig))
		}
	}()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

// readLine reads one line from the given reader.
func readLine(in io.Reader) (string, error) {
	str, err := bufio.NewReader(in).ReadString('\n')
	return strings.TrimSpace(str), err
}

// lineBreakCommand returns a new instance of the lineBreakCommand every time to avoid
// data races in concurrent tests exercising commands.
func lineBreakCommand() *cobra.Command {
	return &cobra.Command{Run: func(*cobra.Command, []string) {}}
}

// withUsage wraps a PositionalArgs to display usage only when the PositionalArgs
// variant is violated.
func withUsage(inner cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := inner(cmd, args); err != nil {
			cmd.Root().SilenceUsage = false
			cmd.SilenceUsage = false
			return err
		}

		return nil
	}
}
