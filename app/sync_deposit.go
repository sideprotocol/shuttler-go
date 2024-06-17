package app

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"go.uber.org/zap"

	btcbridge "github.com/sideprotocol/side/x/btcbridge/types"
)

// Scan the transanctions in the block
// Check if the transaction is a deposit transaction
// If it is, submit the transaction to the sidechain
// block should be confirmed
func (a *State) ScanVaultTx(current int32) error {

	height := current - a.params.Confirmations
	if height == current {
		height = current - 5
	}

	a.Log.Info("Scanning block", zap.Int32("height", height), zap.Int32("current", current))
	// _, bestHeight, err := a.()
	// if err != nil {
	// 	return err
	// }
	lightClientTip, err := a.QueryChainTip()
	if err != nil {
		a.Log.Error("Failed to query light client chain tip", zap.Error(err))
		return nil
	}

	// check if the block is already confirmed
	// if not, return, because sidechain is instant finality,
	// have to wait for the block to be confirmed
	if height < int32(lightClientTip.Height)-a.params.Confirmations {
		return nil
	}

	// process block
	blockhash, err := a.rpc.GetBlockHash(int64(height))
	if err != nil {
		return err
	}
	block, err := a.rpc.GetBlock(blockhash)
	if err != nil {
		return err
	}
	uBlock := btcutil.NewBlock(block)
	for i, tx := range uBlock.Transactions() {
		// check if the transaction is a withdraw transaction
		// check if the transaction is spending from the vault
		// Submit the transaction to the sidechain
		a.Log.Debug("Checking if the transaction is a withdraw transaction", zap.Int("index", i), zap.String("tx", tx.Hash().String()))

		if len(tx.MsgTx().TxIn) > 0 && len(tx.MsgTx().TxIn[0].Witness) == 2 {
			senderPubKey := tx.MsgTx().TxIn[0].Witness[1]

			vault := btcbridge.SelectVaultByPubKey(a.params.Vaults, hex.EncodeToString(senderPubKey))
			if vault == nil {
				break
			}

			err = a.SubmitWithdrawalTx(blockhash, tx, uBlock.Transactions())
			if err != nil {
				return err
			}
		}

		// check if the transaction is a deposit transaction
		for _, txOut := range tx.MsgTx().TxOut {

			pkScript, err := txscript.ParsePkScript(txOut.PkScript)
			if err != nil {
				continue
			}
			addr, err := pkScript.Address(a.GetChainCfg())
			if err != nil {
				continue
			}

			vault := btcbridge.SelectVaultByBitcoinAddress(a.params.Vaults, addr.String())
			if vault == nil {
				continue
			}

			err = a.SubmitDepositTx(blockhash, tx, uBlock.Transactions())
			if err != nil {
				return err
			}
		}

	}
	return nil
}

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

	return a.SendSideTx(depositTx)
}
