package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	zmqclient "github.com/ordishs/go-bitcoin"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	auth "github.com/cosmos/cosmos-sdk/x/auth/types"
	btclightclient "github.com/sideprotocol/side/x/btclightclient/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	DefaultTimeout = 100 * time.Second
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

	// Cosmos Config
	TxFactory       tx.Factory
	GRpcConn        *grpc.ClientConn
	grpcQueryClient btclightclient.QueryClient
	txServiceClient txtypes.ServiceClient
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

// Initialize the application state
// This function is called by the root command before executing any subcommands.
// and should not be called for `init` and `version` commands.
func (a *State) Init() error {
	// Load the configuration file
	err := a.loadConfigFile(context.Background())
	if err != nil {
		return err
	}

	// Set up a connection to the server.
	conn, err := grpc.Dial(a.Config.Side.GRPC, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return err
	}

	// Create a transaction service client
	a.GRpcConn = conn
	a.txServiceClient = txtypes.NewServiceClient(conn)
	a.grpcQueryClient = btclightclient.NewQueryClient(conn)

	if a.Log == nil {
		a.InitLogger(a.Config.Global.LogLevel)
	}

	a.initTxFactory()

	return nil
}

// Query Light Client Chain Tip
func (a *State) QueryChainTip() (*btclightclient.QueryChainTipResponse, error) {
	// Timeout context for our queries
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	res, err := a.grpcQueryClient.QueryChainTip(ctx, &btclightclient.QueryChainTipRequest{})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Query Cosmos Account Auth Info
func (a *State) queryAccountInfo() (*auth.BaseAccount, error) {

	// Query account info
	query := auth.NewQueryClient(a.GRpcConn)
	res, err := query.AccountInfo(context.Background(), &auth.QueryAccountInfoRequest{Address: a.Config.Side.Sender})
	if err != nil {
		return nil, err
	}
	return res.Info, nil

}

// SendTx sends a transaction to the sidechain
func (a *State) SendSideTx(msg sdk.Msg) error {
	// Encode the message
	// create a new encoding config
	encodingConfig := MakeEncodingConfig()
	txBuilder := encodingConfig.TxConfig.NewTxBuilder()
	txBuilder.SetGasLimit(a.Config.Side.Gas)
	txBuilder.SetFeeAmount(sdk.Coins{sdk.NewInt64Coin("uside", int64(2000))})
	txBuilder.SetMsgs(msg)

	// Sign the transaction
	// Query Account info
	account, err := a.queryAccountInfo()
	if err != nil {
		return err
	}

	// Create Signing Factory
	txf := a.TxFactory
	// txf = txf.WithGasPrices("0.00001uside")
	// txf = txf.WithFees("2000uside")
	txf = txf.WithFeePayer(account.GetAddress())
	txf = txf.WithTxConfig(encodingConfig.TxConfig)
	txf = txf.WithAccountNumber(account.AccountNumber)
	txf = txf.WithSequence(account.Sequence)

	err = tx.Sign(txf, InternalKeyringName, txBuilder, true)
	if err != nil {
		log.Fatalf("failed to sign tx: %v", err)
		return err
	}

	txBytes, err := encodingConfig.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		log.Fatalf("failed to encode tx: %v", err)
		return err
	}

	// Broadcast the transaction
	res, err := a.txServiceClient.BroadcastTx(context.Background(), &txtypes.BroadcastTxRequest{
		TxBytes: txBytes,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC, // Change as needed
	})
	if err != nil {
		log.Fatalf("failed to broadcast tx: %v", err)
		return err
	}

	if res.TxResponse.Code != 0 {
		a.Log.Fatal("message failed", zap.String("error", res.TxResponse.RawLog))
		return fmt.Errorf("message failed: %s", res.TxResponse.RawLog)
	}

	fmt.Printf("Transaction broadcasted with TX hash: %s\n", res.TxResponse.TxHash)
	return nil
}

// Send Update Senders Request
func (a *State) SendUpdateSendersRequest(senders []string) error {
	msg := &btclightclient.MsgUpdateSendersRequest{
		Sender:  a.Config.Side.Sender,
		Senders: senders,
	}
	return a.SendSideTx(msg)
}

// Send Submit Block Header Request
func (a *State) SendSubmitBlockHeaderRequest(headers []*btclightclient.BlockHeader) error {
	msg := &btclightclient.MsgSubmitBlockHeaderRequest{
		Sender:       a.Config.Side.Sender,
		BlockHeaders: headers,
	}
	return a.SendSideTx(msg)
}

func (a *State) InitLogger(configLogLevel string) error {
	a.Log = zap.Must(zap.NewDevelopment())
	return nil
}

func (a *State) initTxFactory() {

	cdc := getCodec()
	//create a Keyring
	kb, err := keyring.New(AppName, keyring.BackendTest, a.HomePath, nil, cdc)
	if err != nil {
		panic(err)
	}

	f := tx.Factory{}
	f = f.WithChainID(a.Config.Side.ChainID)
	f = f.WithFromName(InternalKeyringName)
	f = f.WithGas(a.Config.Side.Gas)
	f = f.WithGasAdjustment(1.5)
	f = f.WithKeybase(kb).WithSignMode(signing.SignMode_SIGN_MODE_DIRECT)
	a.TxFactory = f
}

// loadConfigFile reads config file into a.Config if file is present.
func (a *State) loadConfigFile(ctx context.Context) error {

	cb := NewConfigBuilder(a.HomePath)
	// unmarshall them into the wrapper struct
	cfg := cb.LoadConfigFile()
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

	// Get the current height from the sidechain
	lightClientTip, err := a.QueryChainTip()
	if err != nil {
		a.Log.Error("Failed to query light client chain tip", zap.Error(err))
		return
	}

	a.Log.Info("Start syncing light client", zap.Uint64("height", lightClientTip.Height), zap.String("hash", lightClientTip.Hash))

	currentHeight := lightClientTip.Height + 1

	for {
		hash, err := a.rpc.GetBlockHash(int(currentHeight))
		if err != nil {
			a.Log.Error("Failed to process block hash", zap.Error(err))
			return
		}

		if a.LastBitcoinHeader != nil && hash == a.LastBitcoinHeader.Hash {
			a.Synced = true
			a.Log.Info("Reached the last block")
			return
		}

		header, err := a.rpc.GetBlockHeader(hash)
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
		a.SubmitBlock([]*zmqclient.BlockHeader{header})
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

		besthash, err := a.rpc.GetBestBlockHash()
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
		return
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

		b := &btclightclient.BlockHeader{
			PreviousBlockHash: block.PreviousBlockHash,
			Hash:              block.Hash,
			Height:            block.Height,
			Version:           block.Version,
			MerkleRoot:        block.MerkleRoot,
			Time:              block.Time,
			Bits:              block.Bits,
			Nonce:             block.Nonce,
			Ntx:               block.NTx,
		}

		// Submit block to sidechain
		err := a.SendSubmitBlockHeaderRequest([]*btclightclient.BlockHeader{b})
		if err != nil {
			a.Log.Error("Failed to submit block", zap.Error(err))
			panic(err)
		}
	}
}

func (a *State) ReadCA() ([]byte, error) {
	return os.ReadFile(filepath.Join(a.HomePath, CA_FILE))
}

// Close the application state
func (a *State) Close() {
	a.GRpcConn.Close()
}
