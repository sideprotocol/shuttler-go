package app_test

import (
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/sideprotocol/shuttler/app"
	"github.com/stretchr/testify/require"
)

// var hash = bitcoin.HashMerkleBranches

func TestMerkleProof(t *testing.T) {

	cfg := &rpcclient.ConnConfig{
		Host:         "signet:18332",
		User:         "side",
		Pass:         "12345678",
		HTTPPostMode: true,
		DisableTLS:   true,
	}
	client, err := rpcclient.New(cfg, nil)
	if err != nil {
		t.Error(err)
	}

	// bh, err := client.GetBestBlockHash()
	// println("Best block hash:", bh.String())
	// if err != nil {
	// 	t.Error(err)
	// }
	// require.NoError(t, err)
	// require.NotNil(t, bh)

	// 1: 00000000299331190ef55d0543455dc87dbb54d8fd7fd9fc6288f7392795bcc5
	// 2: 0000000086a976245ababae6fd164e892700c3a076f57816160e17f1471a72c6
	// 3: 00000000e2b58534cdc14b69881aa25047d525dcf4efda23d2d6135143dc150c
	// 4: 0000000060ed8d216185cc957e5fa81325815fa4da563ff5300d5754f40e9af5
	// 9: 00000000f50b7f4bd6aa38ba9178d7a708a3d06c50994343452fd06488d7efc2
	// 2000+: 000000000000000c1a466a706ffa3f82dfd7caf9db8796e76eb84b7c226b1926
	// 6000+: 000000000000000d04a51353ad3a630ae352105ffac54a56092084042a5ecb2d

	bh, _ := chainhash.NewHashFromStr("000000000000000c1a466a706ffa3f82dfd7caf9db8796e76eb84b7c226b1926")

	block, err := client.GetBlock(bh)
	if err != nil {
		t.Error(err)
	}
	require.NoError(t, err)
	uBlock := btcutil.NewBlock(block)

	// Target hash to produce a proof for
	index := len(uBlock.Transactions()) - 3
	// index := 0
	if index < 0 {
		index = 0
	}
	hn := uBlock.Transactions()[index] // Change to h1, h2, h4 as needed

	proof := app.GenerateMerkleProof(uBlock.Transactions(), hn.Hash())
	fmt.Println("Merkle Proof for:", hn.Hash().String(), "proof length:", len(proof))

	for i, p := range proof {
		fmt.Printf("Proof %d: %s\n", i+1, p)
	}

	// Verify the proof
	verified := app.VerifyMerkleProof(proof, hn.Hash(), &uBlock.MsgBlock().Header.MerkleRoot)

	// root := blockchain.CalcMerkleRoot(uBlock.Transactions(), false)
	// println("Merkle Root:", uBlock.MsgBlock().Header.MerkleRoot.String(), root.String())
	require.True(t, verified, true)
}
