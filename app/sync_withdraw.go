package app

import (
	"bytes"
	"context"
	"encoding/base64"

	"github.com/btcsuite/btcd/btcutil/psbt"
	btcbridge "github.com/sideprotocol/side/x/btcbridge/types"
	"go.uber.org/zap"
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

func signPSBT(packet *psbt.Packet, wif string) (*psbt.Packet, error) {
	// // Decode the private key
	// privKeyWIF, err := btcutil.DecodeWIF(wif)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to decode WIF: %v", err)
	// }
	// privKey := privKeyWIF.PrivKey

	// // Create a Secp256k1 context
	// secp := btcec.S256()

	// // Sign each input
	// for i := range packet.Inputs {
	// 	txscript.
	// 	sigHashes := txscript.NewTxSigHashes(packet.UnsignedTx, inputfeters)

	// 	// Calculate the signature hash for the input
	// 	sigHash, err := txscript.CalcWitnessSigHash(packet.Inputs[i].WitnessUtxo.PkScript, sigHashes, txscript.SigHashAll, packet.UnsignedTx, i, int64(packet.Inputs[i].WitnessUtxo.Value))
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to calculate signature hash: %v", err)
	// 	}

	// 	// Sign the hash
	// 	signature, err := privKey.Sign(sigHash)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to sign hash: %v", err)
	// 	}

	// 	// Add the signature to the PSBT
	// 	packet.Inputs[i].PartialSigs = append(packet.Inputs[i].PartialSigs, psbt.PartialSig{
	// 		PubKey:    privKey.PubKey().SerializeCompressed(),
	// 		Signature: append(signature.Serialize(), byte(txscript.SigHashAll)),
	// 	})
	// }

	return packet, nil
}
