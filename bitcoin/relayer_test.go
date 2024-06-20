package bitcoin

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/ordishs/go-bitcoin"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	btclightclient "github.com/sideprotocol/side/x/btcbridge/types"
)

// Test Sending Transactions to Cosmos Blockchain
func TestSendingTransactions(t *testing.T) {

	cfg := &rpcclient.ConnConfig{
		Host:         "signet:18332",
		User:         "side",
		Pass:         "12345678",
		HTTPPostMode: true,
		DisableTLS:   true,
	}
	client, err := rpcclient.New(cfg, nil)

	// client, err := bitcoin.New("signet", 18332, "side", "12345678", false)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Replace "targetAddress" with the address you want to check
	targetAddress := "your_target_address"

	// Get the block hash of the latest block
	bestBlockHash, err := client.GetBestBlockHash()
	if err != nil {
		log.Fatalf("Error getting best block hash: %v", err)
	}

	// Get the block info
	block, err := client.GetBlock(bestBlockHash)
	if err != nil {
		log.Fatalf("Error getting block info: %v", err)
	}

	// Check if the target address has transactions in the block
	found := false
	for _, tx := range block.Transactions {
		for _, txOut := range tx.TxOut {
			pkScript, err := txscript.ParsePkScript(txOut.PkScript)
			if err != nil {
				continue
			}
			addr, err := pkScript.Address(&chaincfg.MainNetParams)
			if err != nil {
				continue
			}
			if addr.String() == targetAddress {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if found {
		fmt.Printf("Transaction involving address %s found in block %s\n", targetAddress, bestBlockHash)
	} else {
		fmt.Printf("No transaction involving address %s found in block %s\n", targetAddress, bestBlockHash)
	}

	// Produce a Merkle proof if the target address was found
	// if found {

	// var txs []*btcutil.Tx
	// merkleProof, err := zmq.BuildMerkleTreeStore(txs)
	// if err != nil {
	// 	log.Fatalf("Error producing Merkle proof: %v", err)
	// }
	// fmt.Println("Merkle proof:", merkleProof)
	// }

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
