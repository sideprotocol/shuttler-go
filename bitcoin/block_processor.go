package bitcoin

import (
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/sideprotocol/shuttler/app"
)

// @deprecated
type BTCBlockProcessor struct {
	// Bitcoin client
	client *rpcclient.Client
	app    *app.State
}

func NewBlockProcessor(a *app.State) *BTCBlockProcessor {

	cfg := a.Config
	// ca, err := a.ReadCA() // Read CA certificate
	// if err != nil {
	// 	a.Log.Error("Failed to read CA certificate")
	// }

	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.Bitcoin.RPC,
		User:         cfg.Bitcoin.RPCUser,
		Pass:         cfg.Bitcoin.RPCPassword,
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
		// Endpoint:     "wss", // Endpoint: ws, wss, http, https
		// Certificates: ca,
	}

	// not supported in HTTP POST mode.
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		println(err)
		panic("Failed to create new client")
	}
	// defer client.Shutdown()
	return &BTCBlockProcessor{
		client: client,
		app:    a,
	}
}

// func (b *BTCBlockProcessor) SyncHeader(new_hash string) error {

// 	hash, err := chainhash.NewHashFromStr(new_hash)
// 	if err != nil {
// 		return err
// 	}
// 	// info,err := b.client.GetBlockChainInfo()

// 	block, err := b.client.GetBlock(hash)
// 	// block.
// 	if err != nil {
// 		return err
// 	}

// 	var emptyFlags blockchain.BehaviorFlags
// 	timeSource := blockchain.NewMedianTime()
// 	chainParam := app.ChainParams(b.app.Config.Bitcoin.Chain)

// 	err = blockchain.CheckBlockHeaderSanity(&block.Header, chainParam.PowLimit, timeSource, emptyFlags)
// 	if err != nil {
// 		return err
// 	}

// 	println("Block Hash", block.Header.BlockHash().String())

// 	b.app.Log.Info("Block Hash", zap.Any("header", block.Header))

// 	// // Create a new blockchain instance, no persistent db required for just header checks
// 	// chain, err := blockchain.New(&blockchain.Config{
// 	// 	ChainParams: &chaincfg.MainNetParams,
// 	// })

// 	// if err != nil {
// 	// 	return err
// 	// }

// 	bc := btcutil.NewBlock(block)
// 	powLimit := chaincfg.MainNetParams.PowLimit

// 	b.app.Log.Info("Block height", zap.Int32("height", bc.Height()))

// 	// Verify the block header
// 	err = blockchain.CheckProofOfWork(bc, powLimit)
// 	if err != nil {
// 		return err
// 	}

// 	b.app.TrustHeader = block.Header

// 	return nil
// }

func (b *BTCBlockProcessor) Shutdown() {
	b.client.Shutdown()
	println("Bitcoin relayer shutdown")
}
