package core

// VerifiedTool wraps a standard Tool to explicitly indicate it passed cryptographic checks.
type VerifiedTool struct {
	Tool
	signature string
}

func NewVerifiedTool(t Tool, signatureHex string) *VerifiedTool {
	return &VerifiedTool{Tool: t, signature: signatureHex}
}

func (v *VerifiedTool) Signature() string {
	return v.signature
}
