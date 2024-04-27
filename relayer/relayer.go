package relayer

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sideprotocol/shuttler/app"
)

func Start(a *app.State) {
	// Create a ticker that ticks every second.
	bitcoin := time.NewTicker(1 * time.Second)
	defer bitcoin.Stop()

	// Create another ticker that ticks every 500 milliseconds.
	side := time.NewTicker(500 * time.Millisecond)
	defer side.Stop()

	// Setup a channel to listen for an interrupt or SIGTERM signal.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Using a loop to print "hello" on each tick until a signal is received.
	for {
		select {
		case <-bitcoin.C:
			a.Log.Info("hello")
		case <-side.C:
			a.Log.Info("quick hello")
		case <-sigs:
			a.Log.Info("Exiting...")
			return
		}
	}
	// return nil
}
