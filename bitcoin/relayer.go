package bitcoin

import (
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/sideprotocol/shuttler/app"
	"go.uber.org/zap"
)

type BTCRelayer struct {
	// Bitcoin client
	client *rpcclient.Client
	app    *app.State
}

func NewBTCRelayer(a *app.State) *BTCRelayer {

	cfg := a.Config
	ca, err := a.ReadCA() // Read CA certificate
	if err != nil {
		a.Log.Error("Failed to read CA certificate")
	}

	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.Bitcoin.RPC,
		User:         cfg.Bitcoin.RPCUser,
		Pass:         cfg.Bitcoin.RPCPassword,
		HTTPPostMode: false, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   false, // Bitcoin core does not provide TLS by default
		Endpoint:     "wss", // Endpoint: ws, wss, http, https
		Certificates: ca,
	}

	ntfnHandlers := rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txns []*btcutil.Tx) {
			a.Log.Info("Block connected: %v (%d) %v",
				zap.String("hash", header.BlockHash().String()),
				zap.Int32("height", height),
				zap.Time("timestamp", header.Timestamp),
			)
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			a.Log.Info("Block disconnected: %v (%d) %v",
				zap.String("hash", header.BlockHash().String()),
				zap.Int32("height", height),
				zap.Time("timestamp", header.Timestamp),
			)
		},
	}

	// not supported in HTTP POST mode.
	client, err := rpcclient.New(connCfg, &ntfnHandlers)
	if err != nil {
		println(err)
		panic("Failed to create new client")
	}
	// defer client.Shutdown()
	return &BTCRelayer{
		client: client,
		app:    a,
	}
}

func (b *BTCRelayer) SyncHeader() error {

	height, err := b.client.GetBlockCount()

	if err != nil {
		return err
	}

	println("Best Block Height", height)

	hash, err := b.client.GetBlockHash(height)
	if err != nil {
		return err
	}

	block, err := b.client.GetBlock(hash)
	if err != nil {
		return err
	}

	var emptyFlags blockchain.BehaviorFlags
	timeSource := blockchain.NewMedianTime()
	chainParam := app.ChainParams(b.app.Config.Bitcoin.Chain)

	err = blockchain.CheckBlockHeaderSanity(&block.Header, chainParam.PowLimit, timeSource, emptyFlags)
	if err != nil {
		return err
	}

	println("Block Hash", block.Header.BlockHash().String())

	b.app.Log.Info("Block Hash", zap.Any("header", block.Header))

	// // Create a new blockchain instance, no persistent db required for just header checks
	// chain, err := blockchain.New(&blockchain.Config{
	// 	ChainParams: &chaincfg.MainNetParams,
	// })

	// if err != nil {
	// 	return err
	// }

	bc := btcutil.NewBlock(block)
	powLimit := chaincfg.MainNetParams.PowLimit

	// Verify the block header
	err = blockchain.CheckProofOfWork(bc, powLimit)
	if err != nil {
		return err
	}

	b.app.TrustHeader = block.Header

	return nil
}

func (b *BTCRelayer) Shutdown() {
	b.client.Shutdown()
	println("Bitcoin relayer shutdown")
}
