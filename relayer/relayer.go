package relayer

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sideprotocol/shuttler/app"
	"go.uber.org/zap"

	zmqclient "github.com/ordishs/go-bitcoin"
)

func Start(a *app.State) {

	// Create a new ZMQ client & RPC client
	host := a.Config.Bitcoin.ZMQHost
	port := a.Config.Bitcoin.ZMQPort
	if host == "" || port == 0 {
		panic("ZMQ host or port not set")
	}

	a.Log.Info("Connecting to the Side and Bitcoin network...")
	err := a.InitRPC()
	if err != nil {
		panic(err)
	}

	// 1. Sync the light client with the bitcoin network
	a.FastSyncLightClient()

	// 2. Subscribe to the latest block
	zmq := zmqclient.NewZMQ(host, port)
	btcChan := make(chan []string)
	if err := zmq.Subscribe("hashblock", btcChan); err != nil {
		a.Log.Fatal("%v", zap.Error(err))
	}
	a.Log.Info("Waiting for blocks...")

	// 3. Add a Timer to fetch transactions to the bitcoin network
	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	// Setup a channel to listen for an interrupt or SIGTERM signal.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go handleWithdrawalTxnsLoop(a)

	for {
		select {
		case c := <-btcChan:
			a.OnNewBtcBlock(c)
		case <-sigs:
			a.Log.Info("Exiting...")
			return
		case <-ticker.C:
			// a.SignWithdrawalTxns()
			// a.SyncWithdrawalTxns()
		}
	}
	// return nil
}

func handleWithdrawalTxnsLoop(a *app.State) {
	for {
		a.SignWithdrawalTxns()
		a.SyncWithdrawalTxns()

		time.Sleep(6 * time.Second)
	}
}

// func FetchTxns(a *app.State, client *zmqclient.Bitcoind) {
// 	// Fetch transactions from the bitcoin network
// 	// client.
// }
