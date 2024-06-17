package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"

	"go.uber.org/zap"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"

	btcbridge "github.com/sideprotocol/side/x/btcbridge/types"
)

// SignWithdrawalTxns signs the withdrawal transactions
func (a *State) SignWithdrawalTxns() {

	if !a.Config.Bitcoin.VaultSigner {
		return
	}

	if len(a.Config.Bitcoin.VaultWIF) == 0 {
		a.Log.Error("No WIF found")
		return
	}

	// Timeout context for our queries
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	res, err := a.grpcQueryClient.QuerySigningRequest(ctx, &btcbridge.QuerySigningRequestRequest{
		Status: btcbridge.SigningStatus_SIGNING_STATUS_CREATED,
	})
	if err != nil {
		return
	}

	for _, r := range res.Requests {

		b, err := base64.StdEncoding.DecodeString(r.Psbt)
		if err != nil {
			a.Log.Error("Failed to decode transaction", zap.Error(err))
			continue
		}

		packet, err := psbt.NewFromRawBytes(bytes.NewReader(b), false)
		if err != nil {
			a.Log.Error("Failed to decode transaction", zap.Error(err))
			continue
		}

		packet, err = signPSBT(packet, a.Config.Bitcoin.VaultWIF)
		if err != nil {
			a.Log.Error("Failed to sign transaction", zap.Error(err))
			continue
		}

		w := new(bytes.Buffer)
		err = packet.Serialize(w)
		if err != nil {
			a.Log.Error("Failed to serialize transaction", zap.Error(err))
			continue
		}

		client := btcbridge.NewMsgClient(a.gRPC)
		_, err = client.SubmitWithdrawSignatures(ctx, &btcbridge.MsgSubmitWithdrawSignaturesRequest{
			Sender: a.Config.Side.Sender,
			Txid:   r.Txid,
			Psbt:   base64.StdEncoding.EncodeToString(w.Bytes()),
		})

		if err != nil {
			a.Log.Error("Failed to submit transaction", zap.Error(err))
		}
	}
}

// SyncWithdrawalTxns sends the withdrawal transactions to the bitcoin network
func (a *State) SyncWithdrawalTxns() {
	// Timeout context for our queries
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	res, err := a.grpcQueryClient.QuerySigningRequest(ctx, &btcbridge.QuerySigningRequestRequest{
		Status: btcbridge.SigningStatus_SIGNING_STATUS_SIGNED,
	})
	if err != nil {
		return
	}

	for _, r := range res.Requests {
		b, err := base64.StdEncoding.DecodeString(r.Psbt)
		if err != nil {
			a.Log.Error("Failed to decode transaction", zap.Error(err))
			continue
		}

		packet, err := psbt.NewFromRawBytes(bytes.NewReader(b), false)
		if err != nil {
			a.Log.Error("Failed to decode transaction", zap.Error(err))
			continue
		}

		if !packet.IsComplete() {
			a.Log.Error("Transaction is not complete", zap.String("txid", r.Txid))
			continue
		}

		signedTx, err := psbt.Extract(packet)
		if err != nil {
			a.Log.Error("Failed to extract transaction", zap.Error(err))
			continue
		}

		_, err = a.rpc.SendRawTransaction(signedTx, false)
		if err != nil {
			a.Log.Error("Failed to broadcast transaction", zap.Error(err))
			continue
		}
	}
}

// Submit Withdrawal Transaction to Sidechain
func (a *State) SubmitWithdrawalTx(blockhash *chainhash.Hash, tx *btcutil.Tx, txs []*btcutil.Tx) error {

	// Check if the transaction has at least 1 input
	// If not, it's not a withdrawal transaction
	if len(tx.MsgTx().TxIn) < 1 {
		return nil
	}

	var buf bytes.Buffer
	tx.MsgTx().Serialize(&buf)

	// Calulate the in
	proof := GenerateMerkleProof(txs, tx.Hash())

	withdrawalTx := &btcbridge.MsgSubmitWithdrawTransactionRequest{
		Sender:    a.Config.Side.Sender,
		Blockhash: blockhash.String(),
		TxBytes:   base64.StdEncoding.EncodeToString(buf.Bytes()),
		Proof:     proof,
	}

	a.Log.Debug("Transaction submitted",
		zap.Any("Tx", withdrawalTx),
	)

	return a.SendSideTx(withdrawalTx)
}

func signPSBT(packet *psbt.Packet, wif string) (*psbt.Packet, error) {
	// Decode the private key
	privKeyWIF, err := btcutil.DecodeWIF(wif)
	if err != nil {
		return nil, fmt.Errorf("failed to decode WIF: %v", err)
	}
	privKey := privKeyWIF.PrivKey

	// build previous output fetcher
	prevOutputFetcher := txscript.NewMultiPrevOutFetcher(nil)

	for i, txIn := range packet.UnsignedTx.TxIn {
		prevOutput := packet.Inputs[i].WitnessUtxo
		if prevOutput == nil {
			return nil, fmt.Errorf("witness utxo required")
		}

		prevOutputFetcher.AddPrevOut(txIn.PreviousOutPoint, prevOutput)
	}

	// sign and finalize inputs
	for i := range packet.Inputs {
		output := packet.Inputs[i].WitnessUtxo
		hashType := packet.Inputs[i].SighashType

		witness, err := txscript.WitnessSignature(packet.UnsignedTx, txscript.NewTxSigHashes(packet.UnsignedTx, prevOutputFetcher),
			i, output.Value, output.PkScript, hashType, privKey, true)
		if err != nil {
			return nil, fmt.Errorf("failed to generate witness: %v", err)
		}

		packet.Inputs[i].PartialSigs = append(packet.Inputs[i].PartialSigs, &psbt.PartialSig{
			PubKey:    witness[1],
			Signature: witness[0],
		})

		if err := psbt.Finalize(packet, i); err != nil {
			return nil, fmt.Errorf("failed to finalize: %v", err)
		}
	}

	return packet, nil
}
