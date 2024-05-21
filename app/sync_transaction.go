package app

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
)

// filter block that contain a given address as a recipient
func (a *State) FilterBlockByRecipient(height int64, recipient string) error {
	_, bestHeight, err := a.rpc.GetBestBlock()
	if err != nil {
		return err
	}
	// check if the block is already confirmed
	// if not, return, because sidechain is instant finality,
	// have to wait for the block to be confirmed
	if height > int64(bestHeight-a.params.Confirmations) {
		return nil
	}

	// process block
	blockhash, err := a.rpc.GetBlockHash(height)
	if err != nil {
		return err
	}
	block, err := a.rpc.GetBlock(blockhash)
	if err != nil {
		return err
	}
	uBlock := btcutil.NewBlock(block)
	for _, tx := range uBlock.Transactions() {
		for _, txOut := range tx.MsgTx().TxOut {
			pkScript, err := txscript.ParsePkScript(txOut.PkScript)
			if err != nil {
				continue
			}
			addr, err := pkScript.Address(&chaincfg.MainNetParams)
			if err != nil {
				continue
			}
			if addr.String() == recipient {
				err := a.SubmitDepositTx(blockhash, tx, uBlock.Transactions())
				if err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// Submit Deposit Transaction to Sidechain
func (a *State) SubmitDepositTx(blockhash *chainhash.Hash, tx *btcutil.Tx, txs []*btcutil.Tx) error {

	// Calulate the in
	// proof := GenerateMerkleProof(txs, txHash)

	// depositTx := btclightclient.MsgSubmitTransactionRequest{
	// 	BlockHash: blockhash,
	// 	TxHash:    txHash,
	// 	Proof:     proof,
	// }

	return a.SendSideTx(nil)
}
