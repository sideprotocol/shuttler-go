package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"go.uber.org/zap"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	secpv4 "github.com/decred/dcrd/dcrec/secp256k1/v4"

	crypto "github.com/cosmos/cosmos-sdk/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"

	btcbridge "github.com/sideprotocol/side/x/btcbridge/types"
)

// SignWithdrawalTxns signs the withdrawal transactions
func (a *State) SignWithdrawalTxns() {

	// Ensure local signing is enabled
	if !a.Config.Bitcoin.LocalSigning || len(a.Config.Bitcoin.Vaults) == 0 {
		return
	}

	vaultPrivKeys, err := a.getVaultPrivKeys()
	if err != nil {
		a.Log.Error("Failed to get the vault private keys", zap.Error(err))
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

	a.Log.Info("Syncing withdrawal transactions", zap.Int("count", len(res.Requests)))

	if len(res.Requests) == 0 {
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

		packet, err = signPsbt(packet, vaultPrivKeys)
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

		signingTx := &btcbridge.MsgSubmitWithdrawSignaturesRequest{
			Sender: a.Config.Side.Sender,
			Txid:   r.Txid,
			Psbt:   base64.StdEncoding.EncodeToString(w.Bytes()),
		}

		if err = a.SendSideTxWithRetry(signingTx, a.Config.Side.Retries); err != nil {
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

	a.Log.Info("Syncing withdrawal transactions", zap.Int("count", len(res.Requests)))

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

		signingTx := &btcbridge.MsgSubmitWithdrawStatusRequest{
			Sender: a.Config.Side.Sender,
			Txid:   r.Txid,
			Status: btcbridge.SigningStatus_SIGNING_STATUS_BROADCASTED,
		}

		if err = a.SendSideTxWithRetry(signingTx, a.Config.Side.Retries); err != nil {
			a.Log.Error("Failed to submit transaction", zap.Error(err))
		}
	}
}

// Submit Withdrawal Transaction to Sidechain to close the withdrawal and burn the tokens
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

	return a.SendSideTxWithRetry(withdrawalTx, a.Config.Side.Retries)
}

func (a *State) getVaultPrivKeys() (map[string]*secpv4.PrivateKey, error) {
	privKeys := make(map[string]*secpv4.PrivateKey, len(a.Config.Bitcoin.Vaults))

	for _, vault := range a.Config.Bitcoin.Vaults {
		addr, err := btcutil.DecodeAddress(vault, a.GetChainCfg())
		if err != nil {
			a.Log.Error("Failed to decode address", zap.Error(err))
			return nil, err
		}

		pkScript, err := txscript.PayToAddrScript(addr)
		if err != nil {
			a.Log.Error("Failed to get pk script", zap.Error(err))
			return nil, err
		}

		encrypted, err := a.txFactory.Keybase().ExportPrivKeyArmorByAddress(sdk.MustAccAddressFromBech32(vault), "")
		if err != nil {
			a.Log.Error("Failed to export private key", zap.Error(err))
			return nil, err
		}

		privKey, _, err := crypto.UnarmorDecryptPrivKey(encrypted, "")
		if err != nil {
			a.Log.Error("Failed to decrypt private key", zap.Error(err))
			return nil, err
		}

		privKeys[hex.EncodeToString(pkScript)] = secpv4.PrivKeyFromBytes(privKey.Bytes())
	}

	return privKeys, nil
}

func signPsbt(packet *psbt.Packet, privKeys map[string]*secpv4.PrivateKey) (*psbt.Packet, error) {

	// build previous output fetcher
	prevOutFetcher := txscript.NewMultiPrevOutFetcher(nil)

	for i, txIn := range packet.UnsignedTx.TxIn {
		prevOutput := packet.Inputs[i].WitnessUtxo
		if prevOutput == nil {
			return nil, fmt.Errorf("witness utxo required")
		}

		prevOutFetcher.AddPrevOut(txIn.PreviousOutPoint, prevOutput)
	}

	// sign and finalize inputs
	for i := range packet.Inputs {
		prevOut := packet.Inputs[i].WitnessUtxo
		hashType := packet.Inputs[i].SighashType

		privKey, ok := privKeys[hex.EncodeToString(prevOut.PkScript)]
		if !ok {
			return nil, fmt.Errorf("no key found for input %d", i)
		}

		if err := signPsbtInput(packet, i, prevOut, prevOutFetcher, hashType, privKey); err != nil {
			return nil, err
		}

		if err := psbt.Finalize(packet, i); err != nil {
			return nil, fmt.Errorf("failed to finalize input %d: %v", i, err)
		}
	}

	return packet, nil
}

// signPsbtInput signs the given psbt input
// make sure that the input is witness type
func signPsbtInput(packet *psbt.Packet, idx int, prevOut *wire.TxOut, prevOutFetcher txscript.PrevOutputFetcher, hashType txscript.SigHashType, privKey *secpv4.PrivateKey) error {
	switch {
	case txscript.IsPayToWitnessPubKeyHash(prevOut.PkScript):
		// native segwit
		witness, err := txscript.WitnessSignature(packet.UnsignedTx, txscript.NewTxSigHashes(packet.UnsignedTx, prevOutFetcher),
			idx, prevOut.Value, prevOut.PkScript, hashType, privKey, true)
		if err != nil {
			return fmt.Errorf("failed to generate witness: %v", err)
		}

		packet.Inputs[idx].PartialSigs = append(packet.Inputs[idx].PartialSigs, &psbt.PartialSig{
			PubKey:    witness[1],
			Signature: witness[0],
		})

	case txscript.IsPayToTaproot(prevOut.PkScript):
		// taproot
		witness, err := txscript.TaprootWitnessSignature(packet.UnsignedTx, txscript.NewTxSigHashes(packet.UnsignedTx, prevOutFetcher),
			idx, prevOut.Value, prevOut.PkScript, hashType, privKey)
		if err != nil {
			return fmt.Errorf("failed to generate witness: %v", err)
		}

		packet.Inputs[idx].TaprootKeySpendSig = witness[0]

	default:
		return fmt.Errorf("not supported script type")
	}

	return nil
}
