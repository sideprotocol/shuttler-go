package relayer

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sideprotocol/shuttler/app"
	"go.uber.org/zap"

	zmqclient "github.com/ordishs/go-bitcoin"
)

func Start(a *app.State) {

	// Create a new ZMQ client
	// Subscribe to the hashblock topic
	host := a.Config.Bitcoin.ZMQHost
	port := a.Config.Bitcoin.ZMQPort
	if host == "" || port == 0 {
		panic("ZMQ host or port not set")
	}
	zmq := zmqclient.NewZMQ(host, port)
	client, _ := zmqclient.New("signet", 18332, a.Config.Bitcoin.RPCUser, a.Config.Bitcoin.RPCPassword, false)

	btcChan := make(chan []string)

	// if err := zmq.Subscribe("rawblock", btcChan); err != nil {
	// 	a.Log.Fatal("%v", zap.Error(err))
	// }
	if err := zmq.Subscribe("hashblock", btcChan); err != nil {
		a.Log.Fatal("%v", zap.Error(err))
	}
	a.Log.Info("Waiting for blocks...")

	// Setup a channel to listen for an interrupt or SIGTERM signal.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Process messages
	// btcProcessor := btc.NewBlockProcessor(a)
	//
	FastSyncLightClient(a, client)

	for {
		select {
		case c := <-btcChan:
			OnNewBtcBlock(a, client, c)
		case <-sigs:
			a.Log.Info("Exiting...")
			return
		}
	}
	// return nil
}

// Sync the light client with the bitcoin network
func FastSyncLightClient(a *app.State, client *zmqclient.Bitcoind) {
	// Get the current height from the sidechain
	// TODO: Implement this later
	currentHeight := 2813800
	for {
		hash, err := client.GetBlockHash(currentHeight)
		if err != nil {
			a.Log.Error("Failed to process block hash", zap.Error(err))
			return
		}

		if hash == a.LastBitcoinBlockHash {
			a.Log.Info("Reached the last block")
			return
		}

		block, err := client.GetBlockHeader(hash)
		if err != nil {
			a.Log.Error("Failed to process block", zap.Error(err))
			return
		}

		if a.LastBitcoinBlockHash != "" && a.LastBitcoinBlockHash != block.PreviousBlockHash {
			a.Log.Error("There must be a forked branch", zap.String("lasthash", a.LastBitcoinBlockHash), zap.String("previoushash", block.PreviousBlockHash))
			return
		}

		// a.Log.Info("Submit Block to Sidechain", zap.String("hash", block.Hash))
		// Submit block to sidechain
		// a.SubmitBlock(block)
		a.Log.Debug("Block submitted",
			zap.Uint64("Height", block.Height),
			zap.String("PreviousBlockHash", block.PreviousBlockHash),
			zap.String("MerkleRoot", block.MerkleRoot),
			zap.Uint64("Nonce", block.Nonce),
			zap.String("Bits", block.Bits),
			// zap.Int64("Version", block.Version),
			zap.Uint64("Time", block.Time),
			zap.Uint64("TxCount", block.NTx),
		)

		a.LastBitcoinBlockHash = block.Hash

		besthash, err := client.GetBestBlockHash()
		if besthash == block.Hash || err != nil {
			a.Log.Info("Reached the best block")
			return
		}

		currentHeight++
	}
}

func OnNewBtcBlock(a *app.State, client *zmqclient.Bitcoind, c []string) {
	hash := c[1]

	a.Log.Info("Received block", zap.String("hash", hash))
	block, err := client.GetBlockHeader(hash)
	if err != nil {
		a.Log.Error("Failed to process block", zap.Error(err))
		return
	}

	if a.LastBitcoinBlockHash != block.PreviousBlockHash {
		a.Log.Error("Light Client is out of sync or a forked branch detected",
			zap.Uint64("height", block.Height),
			zap.String("lasthash", a.LastBitcoinBlockHash),
			zap.String("previoushash", block.PreviousBlockHash))
		return
	}

	// a.Log.Info("Submit Block to Sidechain", zap.String("hash", hash))
	// Submit block to sidechain
	// a.SubmitBlock(block)
	a.Log.Debug("Block submitted",
		zap.Uint64("Height", block.Height),
		zap.String("PreviousBlockHash", block.PreviousBlockHash),
		zap.String("MerkleRoot", block.MerkleRoot),
		zap.Uint64("Nonce", block.Nonce),
		zap.String("Bits", block.Bits),
		// zap.Int64("Version", block.Version),
		zap.Uint64("Time", block.Time),
		zap.Uint64("TxCount", block.NTx),
	)

	a.LastBitcoinBlockHash = hash

}

func FetchTxns(a *app.State, client *zmqclient.Bitcoind) {
	// Fetch transactions from the bitcoin network
	// client.
}
