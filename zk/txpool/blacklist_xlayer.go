// blacklist_xlayer.go
package txpool

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/types"
	"github.com/ledgerwatch/erigon/rlp"
)

// IsTransferFromForBlockedAddress checks if a transaction is a transferFrom call with a blocked address as the from parameter
func IsTransferFromForBlockedAddress(blockedList common.OrderedList[common.Address], txn *types.TxSlot) bool {

	if txn.Creation || txn.To == (common.Address{}) {
		return false
	}

	transferFromSig := []byte{0x23, 0xb8, 0x72, 0xdd}

	data, err := getTxData(txn)
	if err != nil {
		return false
	}

	if len(data) < 4 {
		return false
	}

	methodID := data[:4]

	if !bytes.Equal(methodID, transferFromSig) {
		return false
	}

	if len(data) < 36 {
		return false
	}

	fromParam := common.BytesToAddress(data[4+12 : 4+32])

	return blockedList.Contains(fromParam)
}

// getTxData extracts the data field from a transaction based on its type
func getTxData(tx *types.TxSlot) ([]byte, error) {
	switch tx.Type {
	case 0x00: // Legacy Transaction
		var txFields []interface{}
		if err := rlp.DecodeBytes(tx.Rlp, &txFields); err != nil {
			return nil, err
		}
		if len(txFields) < 6 {

			return nil, errors.New("invalid RLP data")
		}
		data, ok := txFields[5].([]byte)
		if !ok {

			return nil, errors.New("no valid data field")
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported tx type: %d", tx.Type)
	}
}
