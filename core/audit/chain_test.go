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

func TestChainManager_AppendLinkage(t *testing.T) {
	cm := NewChainManager()
	event := AuditEvent{Type: "AUTH_LOGIN", Actor: "user1"}
	cm.Append(event)

	if len(cm.blocks) != 2 {
		t.Fatalf("expected chain length 2")
	}

	b1 := cm.blocks[1]
	b0 := cm.blocks[0]

	if b1.PreviousHash != b0.Hash {
		t.Fatalf("cryptographic broken link detected between blocks 0 and 1")
	}
}

func TestChainManager_VerifyChain(t *testing.T) {
	cm := NewChainManager()
	cm.Append(AuditEvent{Type: "EVENT_1"})
	cm.Append(AuditEvent{Type: "EVENT_2"})

	ok, err := cm.VerifyChain()
	if !ok || err != nil {
		t.Fatalf("Expected valid, untampered chain to verify")
	}

	// Tamper!
	cm.blocks[1].Event.Actor = "hacker"

	ok, err = cm.VerifyChain()
	if ok || err == nil {
		t.Fatalf("Expected VerifyChain to detect deep-link tampering")
	}
}
