package bitcoin

import (
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/sideprotocol/shuttler/app"
)

type BTCRelayer struct {
	// Bitcoin client
	config app.Config
	client *rpcclient.Client
	app    *app.State
}

func NewBTCRelayer(cfg app.Config, a *app.State) *BTCRelayer {
	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.Bitcoin.RPC,
		User:         cfg.Bitcoin.RPCUser,
		Pass:         cfg.Bitcoin.RPCPassword,
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		a.Log.Error("Failed to create new client")
	}
	// defer client.Shutdown()
	return &BTCRelayer{
		config: cfg,
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

	return nil
}

func (b *BTCRelayer) Shutdown() {
	b.client.Shutdown()
	println("Bitcoin relayer shutdown")
}
