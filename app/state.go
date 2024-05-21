package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/cosmos/cosmos-sdk/types/tx/signing"
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
// App connects both the bitcoin and cosmos network
// Connect to the bitcoin network via RPC; txindex, zmq must be enabled
// Connect to the cosmos network via gRPC
type State struct {
	// General application state
	// Log is the root logger of the application.
	// Consumers are expected to store and use local copies of the logger
	// after modifying with the .With method.
	Log *zap.Logger

	Viper *viper.Viper

	HomePath string
	Debug    bool
	Config   *Config

	// Bitcoin Variables
	// Last Bitcoin Block
	lastBitcoinBlock *btcjson.GetBlockHeaderVerboseResult
	// Side chain synced to the bitcoin network
	synced bool
	rpc    *rpcclient.Client

	// Cosmos Variables
	account *auth.BaseAccount
	params  *btclightclient.Params
	// TrustHeader     wire.BlockHeader
	txFactory       tx.Factory
	gRPC            *grpc.ClientConn
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
		synced:   false,
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
	a.gRPC = conn
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

// Query Parameters of Light Client
func (a *State) QueryAndCheckLightClientPermission() (*btclightclient.QueryParamsResponse, error) {
	// Timeout context for our queries
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	res, err := a.grpcQueryClient.QueryParams(ctx, &btclightclient.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}

	// Check if the sender is in the list of authorized senders
	authorized := false
	for _, sender := range res.Params.Senders {
		if sender == a.Config.Side.Sender {
			authorized = true
			break
		}
	}

	if !authorized {
		panic(fmt.Sprintf("\n\nYou (%s) are not authorized to send bitcoin blocks to the sidechain.", a.Config.Side.Sender))
	}

	a.params = &res.Params
	return res, nil
}

// Query Sequence of Side Account
func (a *State) QuerySequence() (uint64, error) {
	// Query account info
	account, err := a.queryAccountInfo()
	if err != nil {
		return 0, err
	}
	return account.Sequence, nil
}

// Query Cosmos Account Auth Info
// Sequence number is incremented for each transaction
func (a *State) queryAccountInfo() (*auth.BaseAccount, error) {

	// Return the account if it's already loaded
	// Increment the sequence number for each transaction
	if a.account != nil {
		a.account.Sequence++
		return a.account, nil
	}

	// Query account info
	query := auth.NewQueryClient(a.gRPC)
	res, err := query.AccountInfo(context.Background(), &auth.QueryAccountInfoRequest{Address: a.Config.Side.Sender})
	if err != nil {
		return nil, err
	}
	a.account = res.Info
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
	txf := a.txFactory
	txf = txf.WithGasPrices("0.00001uside")
	txf = txf.WithFees("2000uside")
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
	a.txFactory = f
}

// loadConfigFile reads config file into a.Config if file is present.
func (a *State) loadConfigFile(_ context.Context) error {

	cb := NewConfigBuilder(a.HomePath)
	// unmarshall them into the wrapper struct
	cfg := cb.LoadConfigFile()
	a.Config = cfg

	return nil
}

func (a *State) InitRPC() error {

	a.QueryAndCheckLightClientPermission()

	client, err := rpcclient.New(&rpcclient.ConnConfig{
		Host:         a.Config.Bitcoin.RPC,
		User:         a.Config.Bitcoin.RPCUser,
		Pass:         a.Config.Bitcoin.RPCPassword,
		HTTPPostMode: true,
		DisableTLS:   true,
	}, nil)
	if err != nil {
		return err
	}
	a.rpc = client

	return nil
}

// Close the application state
func (a *State) Close() {
	a.gRPC.Close()
}
