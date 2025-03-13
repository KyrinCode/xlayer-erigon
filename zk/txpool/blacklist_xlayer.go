package txpool

import (
	"bytes"
	"errors"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/types"
)

// Constants
var (
	// transferFrom method signature: 0x23b872dd
	transferFromSig = []byte{0x23, 0xb8, 0x72, 0xdd}

	// Error definitions
	errInvalidRLPFormat = errors.New("invalid RLP format")
	errRLPDataTooShort  = errors.New("RLP data too short")
	errExpectedString   = errors.New("expected string, got list")
)

// IsTransferFromForBlockedAddress checks if a transaction is a transferFrom call with a blocked address as the from parameter
func IsTransferFromForBlockedAddress(txn *types.TxSlot, blockedList common.OrderedList[common.Address]) bool {
	if txn.Creation || txn.To == (common.Address{}) {
		return false
	}

	fromParam, isTransferFrom := extractFromParam(txn.Rlp, txn.Type)
	if !isTransferFrom {
		return false
	}

	return blockedList.Contains(fromParam)
}

// extractFromParam extracts the from parameter from transaction RLP data
// Returns the from parameter and whether it's a transferFrom method
func extractFromParam(rlpData []byte, txType byte) (common.Address, bool) {
	var dataField []byte
	var err error

	switch txType {
	case 0x00: // Legacy Transaction
		dataField, err = extractDataFieldDirectlyFromLegacyTx(rlpData)
	case 0x02: // EIP-1559 Transaction
		dataField, err = extractDataFieldDirectlyFromEIP1559Tx(rlpData[1:])
	default:
		return common.Address{}, false
	}

	if err != nil {
		return common.Address{}, false
	}

	if len(dataField) < 4 {
		return common.Address{}, false
	}

	methodID := dataField[:4]

	if !bytes.Equal(methodID, transferFromSig) {
		return common.Address{}, false
	}

	if len(dataField) < 36 {
		return common.Address{}, false
	}

	fromParam := common.BytesToAddress(dataField[4+12 : 4+32])

	return fromParam, true
}

// extractDataFieldDirectlyFromLegacyTx extracts the data field directly from Legacy transaction RLP data
// Legacy transaction RLP format: [nonce, gasPrice, gasLimit, to, value, data, v, r, s]
func extractDataFieldDirectlyFromLegacyTx(rlpData []byte) ([]byte, error) {
	if len(rlpData) == 0 || rlpData[0] < 0xc0 {
		return nil, errInvalidRLPFormat
	}

	// Skip RLP list prefix
	var pos int
	if rlpData[0] <= 0xf7 {
		pos = 1
	} else {
		lenOfLen := int(rlpData[0] - 0xf7)
		if 1+lenOfLen > len(rlpData) {
			return nil, errRLPDataTooShort
		}
		pos = 1 + lenOfLen
	}

	// Skip first 5 fields (nonce, gasPrice, gasLimit, to, value)
	for i := 0; i < 5; i++ {
		if pos >= len(rlpData) {
			return nil, errRLPDataTooShort
		}

		pos = skipRLPField(rlpData, pos)
		if pos < 0 {
			return nil, errRLPDataTooShort
		}
	}

	// Now pos points to the 6th field (data)
	if pos >= len(rlpData) {
		return nil, errRLPDataTooShort
	}

	return decodeRLPString(rlpData, pos)
}

// extractDataFieldDirectlyFromEIP1559Tx extracts the data field directly from EIP-1559 transaction RLP data
// EIP-1559 transaction RLP format: [chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit, to, value, data, accessList, v, r, s]
func extractDataFieldDirectlyFromEIP1559Tx(rlpData []byte) ([]byte, error) {
	if len(rlpData) == 0 || rlpData[0] < 0xc0 {
		return nil, errInvalidRLPFormat
	}

	// Skip RLP list prefix
	var pos int
	if rlpData[0] <= 0xf7 {
		// Short list
		pos = 1
	} else {
		// Long list
		lenOfLen := int(rlpData[0] - 0xf7)
		if 1+lenOfLen > len(rlpData) {
			return nil, errRLPDataTooShort
		}
		pos = 1 + lenOfLen
	}

	// Skip first 7 fields (chainId, nonce, maxPriorityFeePerGas, maxFeePerGas, gasLimit, to, value)
	for i := 0; i < 7; i++ {
		if pos >= len(rlpData) {
			return nil, errRLPDataTooShort
		}

		pos = skipRLPField(rlpData, pos)
		if pos < 0 {
			return nil, errRLPDataTooShort
		}
	}

	// Now pos points to the 8th field (data)
	if pos >= len(rlpData) {
		return nil, errRLPDataTooShort
	}

	// Parse data field
	return decodeRLPString(rlpData, pos)
}

// skipRLPField skips an RLP field and returns the position of the next field
// Returns -1 if data is insufficient
func skipRLPField(data []byte, pos int) int {
	if pos >= len(data) {
		return -1
	}

	// Skip current field based on RLP encoding rules
	if data[pos] < 0x80 {
		// Single byte
		return pos + 1
	} else if data[pos] <= 0xb7 {
		// Short string
		length := int(data[pos] - 0x80)
		if pos+1+length > len(data) {
			return -1
		}
		return pos + 1 + length
	} else if data[pos] <= 0xbf {
		// Long string
		lenOfLen := int(data[pos] - 0xb7)
		if pos+1+lenOfLen > len(data) {
			return -1
		}
		length := 0
		for j := 0; j < lenOfLen; j++ {
			length = length*256 + int(data[pos+1+j])
		}
		if pos+1+lenOfLen+length > len(data) {
			return -1
		}
		return pos + 1 + lenOfLen + length
	} else if data[pos] <= 0xf7 {
		// Short list
		length := int(data[pos] - 0xc0)
		if pos+1+length > len(data) {
			return -1
		}
		return pos + 1 + length
	} else {
		// Long list
		lenOfLen := int(data[pos] - 0xf7)
		if pos+1+lenOfLen > len(data) {
			return -1
		}
		length := 0
		for j := 0; j < lenOfLen; j++ {
			length = length*256 + int(data[pos+1+j])
		}
		if pos+1+lenOfLen+length > len(data) {
			return -1
		}
		return pos + 1 + lenOfLen + length
	}
}

// decodeRLPString decodes an RLP string and returns its content
func decodeRLPString(data []byte, pos int) ([]byte, error) {
	if pos >= len(data) {
		return nil, errRLPDataTooShort
	}

	if data[pos] < 0x80 {
		// Single byte
		return []byte{data[pos]}, nil
	} else if data[pos] <= 0xb7 {
		// Short string
		length := int(data[pos] - 0x80)
		if pos+1+length > len(data) {
			return nil, errRLPDataTooShort
		}
		return data[pos+1 : pos+1+length], nil
	} else if data[pos] <= 0xbf {
		// Long string
		lenOfLen := int(data[pos] - 0xb7)
		if pos+1+lenOfLen > len(data) {
			return nil, errRLPDataTooShort
		}
		length := 0
		for j := 0; j < lenOfLen; j++ {
			length = length*256 + int(data[pos+1+j])
		}
		if pos+1+lenOfLen+length > len(data) {
			return nil, errRLPDataTooShort
		}
		return data[pos+1+lenOfLen : pos+1+lenOfLen+length], nil
	} else {
		// List, not a string
		return nil, errExpectedString
	}
}
