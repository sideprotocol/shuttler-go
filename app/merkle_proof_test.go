package app_test

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/sideprotocol/shuttler/app"
	"github.com/stretchr/testify/require"

	btcbridge "github.com/sideprotocol/side/x/btcbridge/types"
)

// var hash = bitcoin.HashMerkleBranches

func TestMerkleProof(t *testing.T) {

	cfg := &rpcclient.ConnConfig{
		Host:         "signet:38332",
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

	bh, _ := chainhash.NewHashFromStr("000001d36a0074bd4ec73f19dadc6a2df1c7b049daff568e0346c06ea1297e8e")

	block, err := client.GetBlock(bh)
	if err != nil {
		t.Error(err)
	}
	require.NoError(t, err)
	uBlock := btcutil.NewBlock(block)

	// Target hash to produce a proof for
	index := 1
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
	verified := btcbridge.VerifyMerkleProof(proof, hn.Hash(), &uBlock.MsgBlock().Header.MerkleRoot)

	// root := blockchain.CalcMerkleRoot(uBlock.Transactions(), false)
	// println("Merkle Root:", uBlock.MsgBlock().Header.MerkleRoot.String(), root.String())
	require.True(t, verified, true)
}

func TestVerifyMerkleProof(t *testing.T) {
	blockhash := "000001d36a0074bd4ec73f19dadc6a2df1c7b049daff568e0346c06ea1297e8e"
	prev_tx_bytes := "AgAAAAABAevDxNveDbepIJNLRMYvkWnj2MzgyCkIVFQ8F8mi6x/gAAAAAAD9////AnYrdegAAAAAFgAUIjr3Q9KscxPt+jMbXoYTqyyYZ1QAypo7AAAAABYAFKxDJtIA6Z8jP10orEug89GURQ0UAkcwRAIgWiLPeRS7P+sSn6kUPCZ7rG/d7n7g6cBPHZkddpplhlkCIDzDNeYWc9J97EqTMRQZUpXMUwDbf973vhXTvp6R77jvASEDv3a9fVj89tUbSNXT48PaRBv5hyPmifcYzOIyXQcnRdVsDQAA"
	tx_bytes, _ := base64.StdEncoding.DecodeString("AgAAAAABAQSVzKvNWYfNaKcoATilKGaLmVI0CrP4j/ZMs1VMQeQhAAAAAAD9////ArFT2qwAAAAAFgAUB5eO/CjAXpJ2Ae2MLoFGDLX3nYEAypo7AAAAABYAFKxDJtIA6Z8jP10orEug89GURQ0UAkcwRAIgAImsOfNq2DNSIc7yodeghW3eKxMczAjlREkG5Jr3KuUCIH2ziBDp1EQkeJnuaG59FrhhBr3ckqeT/3AVhE6ncRbwASEDeXnjDp/kmpIaas0+MAO6LqXykLlxFonl7Fv1Ar7hsqs/DQAA")
	proof := []string{"AWalPumvpFKZCrC08j75LveGEn3OOVnQUPTYwqPrW5qI"}
	root := "96d5f63826566294ab8b98f18f110c9ecea3bd95839f2af441b63ffea3387e2b"

	println("blockhash:", blockhash)
	println("prev_tx_bytes:", prev_tx_bytes)
	println("tx_bytes:", tx_bytes)
	tx := wire.NewMsgTx(2)
	tx.Deserialize(bytes.NewReader(tx_bytes))
	println("proof:", proof)
	println("root:", root)

	println("tx hash:", tx.TxHash().String())

	bz, _ := hex.DecodeString(root)
	txhash := tx.TxHash()
	rootHash, _ := chainhash.NewHash(bz)
	verfied := btcbridge.VerifyMerkleProof(proof, &txhash, rootHash)
	require.True(t, verfied, true)

	//app.VerifyMerkleProof()

}
