package app

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/wire"
	zmqclient "github.com/ordishs/go-bitcoin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// AppState is the modifiable state of the application.
type State struct {
	// Log is the root logger of the application.
	// Consumers are expected to store and use local copies of the logger
	// after modifying with the .With method.
	Log *zap.Logger

	Viper *viper.Viper

	HomePath    string
	Debug       bool
	Config      *Config
	TrustHeader wire.BlockHeader
	// @deprecated
	// LastBitcoinBlockHash string
	LastBitcoinHeader *zmqclient.BlockHeader
	Synced            bool
	rpc               *zmqclient.Bitcoind
}

// NewState creates a new State object.
func NewAppState(home string) *State {
	h := home
	if h == "" {
		h = DefaultHome
	}
	return &State{
		Viper:    viper.New(),
		HomePath: h,
		Synced:   false,
	}
}

func (a *State) InitLogger(configLogLevel string) error {
	// logLevel := a.Viper.GetString("log-level")
	// if a.Viper.GetBool("debug") {
	// 	logLevel = "debug"
	// } else if logLevel == "" {
	// 	logLevel = configLogLevel
	// }
	// log, err := newRootLogger(a.Viper.GetString("log-format"), logLevel)
	// if err != nil {
	// 	return err
	// }

	a.Log = zap.Must(zap.NewDevelopment())
	return nil
}

// loadConfigFile reads config file into a.Config if file is present.
func (a *State) LoadConfigFile(ctx context.Context) error {

	cb := NewConfigBuilder(a.HomePath)
	// unmarshall them into the wrapper struct
	cfg := cb.LoadConfigFile()

	if a.Log == nil {
		a.InitLogger(cfg.Global.LogLevel)
	}

	a.Config = cfg

	return nil
}

func (a *State) InitRPC() error {

	rpcParam := strings.Split(a.Config.Bitcoin.RPC, ":")
	if len(rpcParam) != 2 {
		return nil
	}
	host := rpcParam[0]
	port, err := strconv.Atoi(rpcParam[1])
	if err != nil {
		return err
	}

	client, err := zmqclient.New(host, port, a.Config.Bitcoin.RPCUser, a.Config.Bitcoin.RPCPassword, false)
	if err != nil {
		return err
	}
	a.rpc = client
	return nil
}

// Sync the light client with the bitcoin network
func (a *State) FastSyncLightClient() {
	client := a.rpc
	// Get the current height from the sidechain
	// TODO: Implement this later
	bestHash, _ := client.GetBestBlockHash()
	bestHeader, _ := client.GetBlockHeader(bestHash)
	currentHeight := bestHeader.Height - 10
	for {
		hash, err := client.GetBlockHash(int(currentHeight))
		if err != nil {
			a.Log.Error("Failed to process block hash", zap.Error(err))
			return
		}

		if a.LastBitcoinHeader != nil && hash == a.LastBitcoinHeader.Hash {
			a.Synced = true
			a.Log.Info("Reached the last block")
			return
		}

		header, err := client.GetBlockHeader(hash)
		if err != nil {
			a.Log.Error("Failed to process block", zap.Error(err))
			return
		}

		if a.LastBitcoinHeader != nil && a.LastBitcoinHeader.Hash != header.PreviousBlockHash {
			a.Log.Error("There must be a forked branch", zap.String("lasthash", a.LastBitcoinHeader.Hash), zap.String("previoushash", header.PreviousBlockHash))
			return
		}

		a.LastBitcoinHeader = header

		// a.Log.Info("Submit Block to Sidechain", zap.String("hash", block.Hash))
		// Submit block to sidechain
		// a.SubmitBlock(block)
		a.Log.Debug("Block submitted",
			zap.Uint64("Height", header.Height),
			zap.String("PreviousBlockHash", header.PreviousBlockHash),
			// zap.String("MerkleRoot", header.MerkleRoot),
			zap.Uint64("Nonce", header.Nonce),
			zap.String("Bits", header.Bits),
			// zap.Int64("Version", block.Version),
			zap.Uint64("Time", header.Time),
			zap.Uint64("TxCount", header.NTx),
		)

		besthash, err := client.GetBestBlockHash()
		if besthash == header.Hash || err != nil {
			a.Synced = true
			a.Log.Info("Reached the best block")
			return
		}

		currentHeight++
	}
}

func (a *State) OnNewBtcBlock(c []string) {
	client := a.rpc
	hash := c[1]

	if !a.Synced {
		a.Log.Info("Not synced yet, skipping block", zap.String("hash", hash))
		return
	}

	// a.Log.Info("Received block", zap.String("hash", hash))
	header, err := client.GetBlockHeader(hash)
	if err != nil {
		a.Log.Error("Failed to process block", zap.Error(err))
		return
	}

	// it's the same block
	if a.LastBitcoinHeader.Hash == header.Hash {
		return
	}

	// Light client is behind the bitcoin network
	if header.Height > a.LastBitcoinHeader.Height+1 {

		a.Log.Info("===================================================================")
		a.Log.Info("Replace the last header with the new one", zap.Uint64("behind", header.Height-a.LastBitcoinHeader.Height))
		a.Log.Info("===================================================================")

		newBlocks := []*zmqclient.BlockHeader{}
		for i := a.LastBitcoinHeader.Height + 1; i < header.Height; i++ {
			hash, err := client.GetBlockHash(int(i))
			if err != nil {
				a.Log.Error("Failed to process block hash", zap.Error(err))
				return
			}

			header, err := client.GetBlockHeader(hash)
			if err != nil {
				a.Log.Error("Failed to process block", zap.Error(err))
				return
			}

			if a.LastBitcoinHeader.Hash != header.PreviousBlockHash {
				a.Log.Error("There must be a forked branch", zap.String("lasthash", a.LastBitcoinHeader.Hash), zap.String("previoushash", header.PreviousBlockHash))
				return
			}

			a.LastBitcoinHeader = header
			newBlocks = append(newBlocks, header)
		}

		a.SubmitBlock(newBlocks)
	}

	// A forked branch detected
	if a.LastBitcoinHeader.Hash != header.PreviousBlockHash {

		a.Log.Error("Forked branch detected",
			zap.Uint64("height", header.Height),
			zap.String("last.hash", a.LastBitcoinHeader.Hash),
			zap.String("last.previoushash", a.LastBitcoinHeader.PreviousBlockHash),
			zap.String("new.hash", header.Hash),
			zap.String("new.previoushash", header.PreviousBlockHash),
		)

		// only check the last one block for now
		// found the the common ancestor, and continue from there.
		if a.LastBitcoinHeader.PreviousBlockHash == header.PreviousBlockHash {
			a.Log.Info("===================================================================")
			a.Log.Info("Replace the last header with the new one", zap.Uint64("height", header.Height))
			a.Log.Info("===================================================================")
			a.LastBitcoinHeader = header

			a.SubmitBlock([]*zmqclient.BlockHeader{header})
			return
		}

		a.Log.Error("Forked branch detected, but no common ancestor found in the last 10 blocks")
		return
	}

	a.SubmitBlock([]*zmqclient.BlockHeader{header})

	a.LastBitcoinHeader = header

}

func (a *State) SubmitBlock(blocks []*zmqclient.BlockHeader) {
	// Submit block to the sidechain
	for i, block := range blocks {
		a.Log.Debug("Block submitted",
			zap.Int("i", i),
			zap.String("P", block.PreviousBlockHash),
			zap.Uint64("Height", block.Height),
			zap.Uint64("v", block.Version),
		)
		a.Log.Debug("Block submitted",
			zap.String("H", block.Hash),
			zap.String("bits", block.Bits),
		)
	}
}

func (a *State) ReadCA() ([]byte, error) {
	return os.ReadFile(filepath.Join(a.HomePath, CA_FILE))
}
