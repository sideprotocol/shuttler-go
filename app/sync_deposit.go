package app

import (
	"bytes"
	"encoding/base64"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"go.uber.org/zap"

	btcbridge "github.com/sideprotocol/side/x/btcbridge/types"
)

// Submit Deposit Transaction to Sidechain
func (a *State) SubmitDepositTx(blockhash *chainhash.Hash, tx *btcutil.Tx, txs []*btcutil.Tx) error {

	// Check if the transaction has at least 1 input
	// If not, it's not a deposit transaction
	if len(tx.MsgTx().TxIn) < 1 {
		return nil
	}

	// Get the previous transaction
	// Use 0th input as the sender
	txIn := tx.MsgTx().TxIn[0]
	prevTxHash := txIn.PreviousOutPoint.Hash
	prevTx, err := a.rpc.GetRawTransaction(&prevTxHash)
	if err != nil {
		return err
	}

	// extract the sender address from the previous transaction
	// Get the address from the previous transaction output
	prevVout := prevTx.MsgTx().TxOut[txIn.PreviousOutPoint.Index]
	_, senderAddrs, _, err := txscript.ExtractPkScriptAddrs(prevVout.PkScript, a.GetChainCfg())
	if err != nil {
		return fmt.Errorf("error extracting addresses: %v", err)
	}
	// Deposit transaction should have only one sender
	if len(senderAddrs) != 1 {
		return fmt.Errorf("deposit transaction should have only one sender: %v", prevTxHash.String())
	}

	// Serialize the transaction
	// Encode the transaction to base64
	var prevBuf bytes.Buffer
	prevTx.MsgTx().Serialize(&prevBuf)

	var buf bytes.Buffer
	tx.MsgTx().Serialize(&buf)

	// Calulate the in
	proof := GenerateMerkleProof(txs, tx.Hash())

	depositTx := &btcbridge.MsgSubmitDepositTransactionRequest{
		Sender:      a.Config.Side.Sender,
		Blockhash:   blockhash.String(),
		PrevTxBytes: base64.StdEncoding.EncodeToString(prevBuf.Bytes()),
		TxBytes:     base64.StdEncoding.EncodeToString(buf.Bytes()),
		Proof:       proof,
	}

	a.Log.Debug("Transaction submitted",
		zap.Any("Tx", depositTx),
	)

	return a.SendSideTxWithRetry(depositTx, a.Config.Side.Retries)
}
