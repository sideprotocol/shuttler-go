package app

import (
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	btclightclient "github.com/sideprotocol/side/x/btclightclient/types"
	"go.uber.org/zap"
)

// Send Submit Block Header Request
func (a *State) SendSubmitBlockHeaderRequest(headers []*btclightclient.BlockHeader) error {
	msg := &btclightclient.MsgSubmitBlockHeaderRequest{
		Sender:       a.Config.Side.Sender,
		BlockHeaders: headers,
	}
	return a.SendSideTx(msg)
}

// Sync the light client with the bitcoin network
func (a *State) FastSyncLightClient() {

	// Get the current height from the sidechain
	lightClientTip, err := a.QueryChainTip()
	if err != nil {
		a.Log.Error("Failed to query light client chain tip", zap.Error(err))
		return
	}

	a.Log.Info("Start syncing light client", zap.Uint64("height", lightClientTip.Height), zap.String("hash", lightClientTip.Hash))

	currentHeight := lightClientTip.Height + 1

	for {
		hash, err := a.rpc.GetBlockHash(int64(currentHeight))
		if err != nil {
			a.Log.Error("Failed to process block hash", zap.Error(err))
			return
		}

		if a.lastBitcoinBlock != nil && hash.String() == a.lastBitcoinBlock.Hash {
			a.synced = true
			a.Log.Info("Reached the last block")
			return
		}

		block, err := a.rpc.GetBlockHeaderVerbose(hash)
		if err != nil {
			a.Log.Error("Failed to process block", zap.Error(err))
			return
		}

		if a.lastBitcoinBlock != nil && a.lastBitcoinBlock.Hash != block.PreviousHash {
			a.Log.Error("There must be a forked branch", zap.String("lasthash", a.lastBitcoinBlock.Hash), zap.String("previoushash", block.PreviousHash))
			return
		}

		a.lastBitcoinBlock = block

		// a.Log.Info("Submit Block to Sidechain", zap.String("hash", block.Hash))
		// Submit block to sidechain
		a.SubmitBlock([]*btcjson.GetBlockHeaderVerboseResult{block})
		a.Log.Debug("Block submitted",
			zap.Int32("Height", block.Height),
			zap.String("PreviousBlockHash", block.PreviousHash),
			// zap.String("MerkleRoot", header.MerkleRoot),
			zap.Uint64("Nonce", block.Nonce),
			zap.String("Bits", block.Bits),
			// zap.Int64("Version", block.Version),
			zap.Int64("Time", block.Time),
			// zap.Int64("TxCount", header.),
		)

		besthash, err := a.rpc.GetBestBlockHash()
		if besthash.String() == block.Hash || err != nil {
			a.synced = true
			a.Log.Info("Reached the best block")
			return
		}

		currentHeight++
	}
}

func (a *State) OnNewBtcBlock(c []string) {
	client := a.rpc
	hash, err := chainhash.NewHashFromStr(c[1])
	if err != nil {
		a.Log.Error("Failed to process block hash", zap.Error(err))
		return
	}

	if !a.synced {
		a.Log.Info("Not synced yet, skipping block", zap.String("hash", hash.String()))
		return
	}

	// a.Log.Info("Received block", zap.String("hash", hash))
	block, err := client.GetBlockHeaderVerbose(hash)
	if err != nil {
		a.Log.Error("Failed to process block", zap.Error(err))
		return
	}

	// it's the same block
	if a.lastBitcoinBlock.Hash == block.Hash {
		return
	}

	// Light client is behind the bitcoin network
	if block.Height > a.lastBitcoinBlock.Height+1 {

		a.Log.Info("===================================================================")
		a.Log.Info("Replace the last header with the new one", zap.Int32("behind", block.Height-a.lastBitcoinBlock.Height))
		a.Log.Info("===================================================================")

		newBlocks := []*btcjson.GetBlockHeaderVerboseResult{}
		for i := a.lastBitcoinBlock.Height + 1; i < block.Height; i++ {
			hash, err := client.GetBlockHash(int64(i))
			if err != nil {
				a.Log.Error("Failed to process block hash", zap.Error(err))
				return
			}

			block, err := client.GetBlockHeaderVerbose(hash)
			if err != nil {
				a.Log.Error("Failed to process block", zap.Error(err))
				return
			}

			if a.lastBitcoinBlock.Hash != block.PreviousHash {
				a.Log.Error("There must be a forked branch", zap.String("lasthash", a.lastBitcoinBlock.Hash), zap.String("previoushash", block.PreviousHash))
				return
			}

			a.lastBitcoinBlock = block
			newBlocks = append(newBlocks, block)
		}

		a.SubmitBlock(newBlocks)
		return
	}

	// A forked branch detected
	if a.lastBitcoinBlock.Hash != block.PreviousHash {

		a.Log.Error("Forked branch detected",
			zap.Int32("height", block.Height),
			zap.String("last.hash", a.lastBitcoinBlock.Hash),
			zap.String("last.previoushash", a.lastBitcoinBlock.PreviousHash),
			zap.String("new.hash", block.Hash),
			zap.String("new.previoushash", block.PreviousHash),
		)

		// only check the last one block for now
		// found the the common ancestor, and continue from there.
		if a.lastBitcoinBlock.PreviousHash == block.PreviousHash {
			a.Log.Info("===================================================================")
			a.Log.Info("Replace the last header with the new one", zap.Int32("height", block.Height))
			a.Log.Info("===================================================================")
			a.lastBitcoinBlock = block

			a.SubmitBlock([]*btcjson.GetBlockHeaderVerboseResult{block})
			return
		}

		a.Log.Error("Forked branch detected, but no common ancestor found in the last 10 blocks")
		return
	}

	a.SubmitBlock([]*btcjson.GetBlockHeaderVerboseResult{block})

	a.lastBitcoinBlock = block

}

func (a *State) SubmitBlock(blocks []*btcjson.GetBlockHeaderVerboseResult) {
	// Submit block to the sidechain
	for i, block := range blocks {
		a.Log.Debug("Block submitted",
			zap.Int("i", i),
			zap.String("P", block.PreviousHash),
			zap.Int32("Height", block.Height),
			zap.Int32("v", block.Version),
		)
		a.Log.Debug("Block submitted",
			zap.String("H", block.Hash),
			zap.String("bits", block.Bits),
		)

		b := &btclightclient.BlockHeader{
			PreviousBlockHash: block.PreviousHash,
			Hash:              block.Hash,
			Height:            uint64(block.Height),
			Version:           uint64(block.Version),
			MerkleRoot:        block.MerkleRoot,
			Time:              uint64(block.Time),
			Bits:              block.Bits,
			Nonce:             uint64(block.Nonce),
			// Ntx:               uint64(block.),
		}

		// Submit block to sidechain
		err := a.SendSubmitBlockHeaderRequest([]*btclightclient.BlockHeader{b})
		if err != nil {
			a.Log.Error("Failed to submit block", zap.Error(err))
			panic(err)
		}
	}
}
