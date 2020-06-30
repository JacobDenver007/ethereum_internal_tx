package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// InternalTx represents contract internla txs.
type InternalTx struct {
	From  common.Address `json:"from"`
	To    common.Address `json:"to"`
	Value *big.Int       `json:"value,omitempty"`
	Depth uint64         `json:"depth,omitempty"`
}
