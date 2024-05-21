package app

import (
	"encoding/base64"
	"math"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

// merkleTreeWidth calculates the width of a Merkle tree
// nTransactions: The number of transactions in the tree
// Returns: The width of layer 0 of the Merkle tree
func merkleTreeWidth(nTransactions int) int {

	n := nTransactions

	// If n is already a power of 2, return it
	if (n & (n - 1)) == 0 {
		return n
	}

	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16 // This line should be extended for 64-bit integers with `n |= n >> 32`

	// The next power of 2 is one more than the number with all ones below MSB
	return n + 1
}

// Calculate the height of a Merkle tree
// n: The number of transactions in the tree
// Returns: The height of the Merkle tree
func merkleTreeHeight(n int) int {
	// Special case for empty tree
	if n == 0 {
		return 0
	}

	// Calculate the height using the formula: height = ceil(log2(n))
	height := int(math.Ceil(math.Log2(float64(n))))

	return height
}

// Function to generate a Merkle proof
func GenerateMerkleProof(txs []*btcutil.Tx, hn *chainhash.Hash) []*string {

	ntx := len(txs)
	if ntx <= 1 {
		return []*string{}
	}

	// Calculate the Merkle tree
	tree := blockchain.BuildMerkleTreeStore(txs, false)

	// Find the index of the hash in the tree
	index := -1
	for i, v := range tree {
		if v == hn || v.IsEqual(hn) {
			index = i
			break
		}
	}
	if index == -1 {
		return nil // Hash not found in the tree
	}

	// Calculate the Merkle proof
	proof := []*string{}
	height := merkleTreeHeight(ntx)
	currentLevelStart := 0
	currentLevelSize := merkleTreeWidth(ntx) // This should be set to the number of leaves initially

	for h := 0; h < height; h++ { // Iterate while there's a higher level

		nextLevelStart := currentLevelStart + currentLevelSize
		// println("Current level start:", currentLevelStart, "Current level size:", currentLevelSize, "Next level start:", nextLevelStart, "Index:", index)
		// relativeIndex := index - currentLevelStart
		var position byte
		siblingIndex := -1
		if index%2 == 0 { // Even index, sibling is next
			siblingIndex = index + 1
			position = byte(0) // sibling is right child
		} else { // Odd index, sibling is previous
			siblingIndex = index - 1
			position = byte(1) // sibling is left child
		}

		siblingIndex += currentLevelStart

		if siblingIndex >= currentLevelStart && siblingIndex < nextLevelStart {
			// println(index, "Node", tree[currentLevelStart+index].String(), "Sibling", tree[siblingIndex].String())
			sibling := tree[siblingIndex]
			var bz []byte
			if sibling == nil {
				bz = []byte{position}
			} else {
				bz = append([]byte{position}, sibling.CloneBytes()...)
			}
			pstr := base64.StdEncoding.EncodeToString(bz)
			proof = append(proof, &pstr)
		}

		// Move to the next level
		index /= 2
		currentLevelSize /= 2
		currentLevelStart = nextLevelStart
	}

	return proof
}

// VerifyMerkleProof verifies a Merkle proof
func VerifyMerkleProof(proofs []*string, hn, root *chainhash.Hash) bool {
	current := hn
	for _, proof := range proofs {
		if proof == nil {
			return false
		}

		bytes, err := base64.StdEncoding.DecodeString(*proof)
		if err != nil {
			return false
		}
		position := bytes[0]
		p := current
		if len(bytes) > 1 {
			p, err = chainhash.NewHash(bytes[1:])
			if err != nil {
				return false
			}
		}

		var temp chainhash.Hash
		if position == 0 {
			temp = blockchain.HashMerkleBranches(current, p)
		} else {
			temp = blockchain.HashMerkleBranches(p, current)
		}
		current = &temp
	}

	return current.IsEqual(root)

}
