package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type ChainManager struct {
	mu     sync.RWMutex
	blocks []Block
}

func generateHash(b Block) string {
	h := sha256.New()
	h.Write([]byte(b.PreviousHash))
	return hex.EncodeToString(h.Sum(nil))
}

func NewChainManager() *ChainManager {
	genesis := Block{
		Index:        0,
		Timestamp:    time.Now(),
		Event:        AuditEvent{Type: "SYSTEM_INIT"},
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
	}
	genesis.Hash = generateHash(genesis)
	return &ChainManager{
		blocks: []Block{genesis},
	}
}
