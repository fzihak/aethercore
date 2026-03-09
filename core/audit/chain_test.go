package audit

import "testing"

func TestChainManager_Init(t *testing.T) {
	cm := NewChainManager()
	if cm == nil {
		t.Fatalf("expected non-nil manager")
	}
}

func TestChainManager_GenesisBlock(t *testing.T) {
	cm := NewChainManager()
	if len(cm.blocks) != 1 {
		t.Fatalf("expected 1 block (Genesis), got %d", len(cm.blocks))
	}
	g := cm.blocks[0]
	if g.Index != 0 || len(g.Hash) != 64 {
		t.Fatalf("invalid genesis block metadata")
	}
}

func TestChainManager_HashSerialization(t *testing.T) {
	cm := NewChainManager()
	event := AuditEvent{Type: "TEST_EVENT", Actor: "system"}
	h := cm.calculateHash(Block{Index: 1, Event: event, PreviousHash: cm.blocks[0].Hash})
	if len(h) != 64 {
		t.Fatalf("expected valid SHA-256 hex string")
	}
}
