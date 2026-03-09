package audit

import "sync"

type ChainManager struct {
	mu     sync.RWMutex
	blocks []Block
}

func NewChainManager() *ChainManager {
	return &ChainManager{}
}
