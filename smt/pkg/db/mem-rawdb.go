package db

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ledgerwatch/erigon/smt/pkg/utils"
)

var (
	RawErrNotFound = fmt.Errorf("key not found")
)

type RawMemDb struct {
	Db          map[[32]byte][]byte
	DbAccVal    map[string][]string
	DbKeySource map[string][]byte
	DbHashKey   map[string][]byte
	DbCode      map[string][]byte
	LastRoot    *big.Int
	Depth       uint8

	lock sync.RWMutex
}

func NewRawMemDb() *RawMemDb {
	return &RawMemDb{
		Db:          make(map[[32]byte][]byte),
		DbAccVal:    make(map[string][]string),
		DbKeySource: make(map[string][]byte),
		DbHashKey:   make(map[string][]byte),
		DbCode:      make(map[string][]byte),
		LastRoot:    big.NewInt(0),
		Depth:       0,
	}
}

func (m *RawMemDb) OpenBatch(quitCh <-chan struct{}) {
}

func (m *RawMemDb) CommitBatch() error {
	return nil
}

func (m *RawMemDb) RollbackBatch() {
}

func (m *RawMemDb) GetLastRoot() (*big.Int, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.LastRoot, nil
}

func (m *RawMemDb) SetLastRoot(value *big.Int) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.LastRoot = value
	return nil
}

func (m *RawMemDb) GetDepth() (uint8, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.Depth, nil
}

func (m *RawMemDb) SetDepth(depth uint8) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.Depth = depth
	return nil
}

func (m *RawMemDb) Get(key utils.NodeKey) (utils.NodeValue12, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	rawVal, err := m.GetRaw(key)
	if err != nil {
		return utils.NodeValue12{}, err
	}
	val, _ := utils.NodeValue12FromRaw(&rawVal)
	return val, nil
}

func (m *RawMemDb) Insert(key utils.NodeKey, value utils.NodeValue12) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	rawVal, err := utils.NodeValue12ToRaw(&value)
	if err != nil {
		return err
	}
	m.InsertRaw(key, rawVal)
	return nil
}

func (m *RawMemDb) GetRaw(key utils.NodeKey) (utils.NodeValue12Raw, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	keyBytes := utils.NodeKeyToByteArray(&key)
	buf := [32]byte{}
	copy(buf[:], keyBytes)
	data, _ := m.Db[buf]

	if data == nil {
		return utils.NodeValue12Raw{}, nil
	}
	val := utils.NodeValue12RawFromByteArray(data)
	return val, nil
}

func (m *RawMemDb) InsertRaw(key utils.NodeKey, value utils.NodeValue12Raw) error {
	m.lock.Lock()         // Lock for writing
	defer m.lock.Unlock() // Make sure to unlock when done
	keyBytes := utils.NodeKeyToByteArray(&key)
	valBytes := utils.NodeValue12RawToByteArray(&value)

	buf := [32]byte{}
	copy(buf[:], keyBytes)
	m.Db[buf] = valBytes
	return nil
}

func (m *RawMemDb) Delete(key string) error {
	return nil
}

func (m *RawMemDb) DeleteByNodeKey(key utils.NodeKey) error {
	return nil
}

func (m *RawMemDb) GetAccountValue(key utils.NodeKey) (utils.NodeValue8, error) {
	m.lock.RLock()         // Lock for reading
	defer m.lock.RUnlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])

	k := utils.ConvertBigIntToHex(keyConc)

	values := utils.NodeValue8{}
	for i, v := range m.DbAccVal[k] {
		values[i] = utils.ConvertHexToBigInt(v)
	}

	return values, nil
}

func (m *RawMemDb) InsertAccountValue(key utils.NodeKey, value utils.NodeValue8) error {
	m.lock.Lock()         // Lock for writing
	defer m.lock.Unlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])
	k := utils.ConvertBigIntToHex(keyConc)

	values := make([]string, 8)
	for i, v := range value {
		values[i] = utils.ConvertBigIntToHex(v)
	}

	m.DbAccVal[k] = values
	return nil
}

func (m *RawMemDb) InsertKeySource(key utils.NodeKey, value []byte) error {
	m.lock.Lock()         // Lock for writing
	defer m.lock.Unlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])

	m.DbKeySource[keyConc.String()] = value
	return nil
}

func (m *RawMemDb) DeleteKeySource(key utils.NodeKey) error {
	m.lock.Lock()         // Lock for writing
	defer m.lock.Unlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])

	delete(m.DbKeySource, keyConc.String())
	return nil
}

func (m *RawMemDb) GetKeySource(key utils.NodeKey) ([]byte, error) {
	m.lock.RLock()         // Lock for reading
	defer m.lock.RUnlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])

	s, ok := m.DbKeySource[keyConc.String()]

	if !ok {
		return nil, ErrNotFound
	}

	return s, nil
}

func (m *RawMemDb) InsertHashKey(key utils.NodeKey, value utils.NodeKey) error {
	m.lock.Lock()         // Lock for writing
	defer m.lock.Unlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])
	k := utils.ConvertBigIntToHex(keyConc)

	valConc := utils.ArrayToScalar(value[:])

	m.DbHashKey[k] = valConc.Bytes()
	return nil
}

func (m *RawMemDb) DeleteHashKey(key utils.NodeKey) error {
	m.lock.Lock()         // Lock for writing
	defer m.lock.Unlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])
	k := utils.ConvertBigIntToHex(keyConc)

	delete(m.DbHashKey, k)
	return nil
}

func (m *RawMemDb) GetHashKey(key utils.NodeKey) (utils.NodeKey, error) {
	m.lock.RLock()         // Lock for reading
	defer m.lock.RUnlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])
	k := utils.ConvertBigIntToHex(keyConc)

	s, ok := m.DbHashKey[k]

	if !ok {
		return utils.NodeKey{}, ErrNotFound
	}

	nv := big.NewInt(0).SetBytes(s)

	na := utils.ScalarToArray(nv)

	return utils.NodeKey{na[0], na[1], na[2], na[3]}, nil
}

func (m *RawMemDb) GetCode(codeHash []byte) ([]byte, error) {
	return []byte{}, nil
}

func (m *RawMemDb) AddCode(code []byte) error {
	return nil
}

func (m *RawMemDb) IsEmpty() bool {
	m.lock.RLock()         // Lock for reading
	defer m.lock.RUnlock() // Make sure to unlock when done

	return len(m.Db) == 0
}

func (m *RawMemDb) PrintDb() {
	//m.lock.RLock()         // Lock for reading
	//defer m.lock.RUnlock() // Make sure to unlock when done
	//
	//for k, v := range m.Db {
	//	println(k, v)
	//}
}

func (m *RawMemDb) GetDb() map[string][]string {
	return make(map[string][]string)
}
