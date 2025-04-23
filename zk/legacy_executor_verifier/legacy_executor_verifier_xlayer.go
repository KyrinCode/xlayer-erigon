package legacy_executor_verifier

import (
	"github.com/ledgerwatch/erigon/zk/smt"
)

func (v *LegacyExecutorVerifier) SetSmtCache(cache *smt.SmtCache) {
	v.cache = cache
}

func minUint64(a, b uint64) uint64 {
	if a <= b {
		return a
	}
	return b
}
