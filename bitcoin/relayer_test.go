package bitcoin

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/ordishs/go-bitcoin"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	btclightclient "github.com/sideprotocol/side/x/btclightclient/types"
)

// Test Sending Transactions to Cosmos Blockchain
func TestSendingTransactions(t *testing.T) {
	// gRPC endpoint of the Cosmos SDK node
	// nodeEndpoint := "localhost:9090"

	// // Setup a gRPC connection to the node
	// conn, err := grpc.Dial(nodeEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	// if err != nil {
	// 	log.Fatalf("failed to dial: %v", err)
	// }
	// defer conn.Close()

	// // Create a transaction service client
	// txClient := txtypes.NewServiceClient(conn)

	// // Creating a simple send message from one account to another
	// fromPrivKey := secp256k1.GenPrivKey()
	// fromAddr := sdk.AccAddress(fromPrivKey.PubKey().Address()).String()
	// toAddr := "cosmos1..." // Change this to the recipient's address

	// // Create a MsgSend
	// msg := &banktypes.MsgSend{
	// 	FromAddress: fromAddr,
	// 	ToAddress:   toAddr,
	// 	Amount:      sdk.NewCoins(sdk.NewInt64Coin("uatom", 100)), // 100 uatom
	// }

	// // Encode the message
	// // create a new encoding config
	// encodingConfig := app.MakeEncodingConfig()

	// Sign the transaction
	// signerData := authtypes.SignerData{
	// 	ChainID:       "chain-id", // Your chain ID here
	// 	AccountNumber: 0,          // Replace with actual account number
	// 	Sequence:      0,          // Replace with actual sequence
	// }
	// tx.NewFactoryCLI(ctx, flag)
	// err = tx.Sign(txBuilder, client.DefaultSignerFactory(encodingConfig.TxConfig.SignModeHandler()), fromPrivKey, signerData, encodingConfig.TxConfig)
	// err = tx.Sign(tx.Factory{}, "name", txBuilder, true)
	// if err != nil {
	// 	log.Fatalf("failed to sign tx: %v", err)
	// }

	// txBytes, err := encodingConfig.TxConfig.TxEncoder()(txBuilder.GetTx())
	// if err != nil {
	// 	log.Fatalf("failed to encode tx: %v", err)
	// }

	// // Broadcast the transaction
	// res, err := txClient.BroadcastTx(context.Background(), &txtypes.BroadcastTxRequest{
	// 	TxBytes: txBytes,
	// 	Mode:    txtypes.BroadcastMode_BROADCAST_MODE_BLOCK, // Change as needed
	// })
	// if err != nil {
	// 	log.Fatalf("failed to broadcast tx: %v", err)
	// }

	// fmt.Printf("Transaction broadcasted with TX hash: %s\n", res.TxResponse.TxHash)
}

// Test Cosmos gRPC client
func Test_Cosmos(t *testing.T) {
	// Create a new Cosmos gRPC client

	// Set up a connection to the server.
	conn, err := grpc.Dial("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	// Create a new client for the BTC service
	client2 := btclightclient.NewQueryClient(conn)

	// Timeout context for our queries
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// btclc.RegisterQueryServer(client, &btclc.QueryServer{})
	// client.
	res, err := client2.QueryChainTip(ctx, &btclightclient.QueryChainTipRequest{})

	if err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("Hash %s Height: %d", res.Hash, res.Height)

}

func Test_Relayer(t *testing.T) {

	client, err := bitcoin.New("signet", 18332, "side", "12345678", false)
	if err != nil {
		t.Fatalf("%v", err)
	}
	latest, err := client.GetBestBlockHash()
	if err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("latest block hash: %v", latest)
	block, err := client.GetBlock(latest)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// print each transaction in the block
	for _, tx := range block.Tx {
		t.Logf("tx: %v", tx)
		rawtx, err := client.GetRawTransactionHex(tx)
		if err != nil {
			t.Fatalf("%v", err)
			break
		}
		t.Logf("rawtx: %v", rawtx)
	}

	// zmq := bitcoin.NewZMQ("signet", 18330)

	// ch := make(chan []string)

	// go func() {
	// 	for c := range ch {
	// 		t.Logf("%v", c)
	// 	}
	// }()

	// // if err := zmq.Subscribe("rawblock", ch); err != nil {
	// // 	t.Fatalf("%v", err)
	// // }

	// if err := zmq.Subscribe("hashblock", ch); err != nil {
	// 	t.Fatalf("%v", err)
	// }

	// t.Log("Waiting for blocks...")

	// waitCh := make(chan bool)
	// <-waitCh

}
