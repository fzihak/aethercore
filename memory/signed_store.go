package memory

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"time"
)

// Sentinel errors for token validation failures.
var (
	ErrTokenExpired = errors.New("signed memory: query token has expired")
	ErrTokenInvalid = errors.New("signed memory: query token signature invalid")
	ErrUnknownNode  = errors.New("signed memory: signer node not in trusted CA pool")
	ErrBadKey       = errors.New("signed memory: certificate key is not ECDSA")
)

// tokenTTL is the maximum age of a valid QueryToken.
const tokenTTL = 30 * time.Second

// QueryToken is a signed, time-limited permission token that authorises one
// query against a remote SignedMemoryStore.
// The Signature covers Payload with a per-request SHA-256 digest signed with
// the issuing node's ECDSA P-256 leaf certificate key.
type QueryToken struct {
	NodeID    string    `json:"node_id"`
	IssuedAt  int64     `json:"issued_at"` // unix nanoseconds
	TopK      int       `json:"top_k"`
	Embedding []float32 `json:"embedding"`
	Signature []byte    `json:"sig"` // ASN.1 DER-encoded ECDSA signature
}

// SignedQueryRequest packages a token with a serialised certificate for
// peer verification over the network.
type SignedQueryRequest struct {
	Token   *QueryToken `json:"token"`
	CertDER []byte      `json:"cert_der"` // leaf DER certificate for signature verification
}

// SignedQueryResponse is the response to a SignedQueryRequest.
type SignedQueryResponse struct {
	Results []MemoryResult `json:"results"`
	Error   string         `json:"error,omitempty"`
}

// SignedMemoryStore wraps a VectorStore and enforces cryptographic access
// control: callers must present a valid QueryToken signed by a trusted peer
// before their query is executed.
type SignedMemoryStore struct {
	store  *VectorStore
	caPool *x509.CertPool // trusts the mesh CA; populated from NodeIdentity
}

// NewSignedMemoryStore creates a signed store backed by the given VectorStore.
// The caPool is used to verify the signing certificates of remote callers.
// Pass identity.TLSConfig.ClientCAs (or RootCAs) as the pool.
func NewSignedMemoryStore(store *VectorStore, caPool *x509.CertPool) *SignedMemoryStore {
	return &SignedMemoryStore{store: store, caPool: caPool}
}

// AuthorisedQuery verifies the QueryToken, then executes the embedded query
// against the underlying VectorStore. Returns ErrTokenExpired or ErrTokenInvalid
// on authentication failure.
func (s *SignedMemoryStore) AuthorisedQuery(req *SignedQueryRequest) ([]MemoryResult, error) {
	token := req.Token

	// 1. Freshness check — reject stale tokens.
	issued := time.Unix(0, token.IssuedAt)
	if time.Since(issued) > tokenTTL {
		return nil, ErrTokenExpired
	}

	// 2. Parse the peer's leaf certificate.
	cert, err := x509.ParseCertificate(req.CertDER)
	if err != nil {
		return nil, fmt.Errorf("signed memory: parse peer cert: %w", err)
	}

	// 3. Verify the peer certificate is signed by the trusted CA.
	opts := x509.VerifyOptions{Roots: s.caPool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	if _, err := cert.Verify(opts); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnknownNode, err)
	}

	// 4. Extract the ECDSA public key from the certificate.
	ecPub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, ErrBadKey
	}

	// 5. Verify the ECDSA signature over the token digest.
	digest := tokenDigest(token)
	if !ecdsa.VerifyASN1(ecPub, digest[:], token.Signature) {
		return nil, ErrTokenInvalid
	}

	// 6. Execute the query.
	return s.store.Query(token.Embedding, token.TopK), nil
}

// SignQuery creates a signed QueryToken using the ECDSA key embedded in the
// TLS certificate from the provided TLS config. The token is ready to send
// in a SignedQueryRequest.
func SignQuery(tlsCfg *tls.Config, embedding []float32, topK int, nodeID string) (*QueryToken, []byte, error) {
	if len(tlsCfg.Certificates) == 0 {
		return nil, nil, errors.New("signed memory: no certificate in TLS config")
	}
	cert := tlsCfg.Certificates[0]

	ecKey, ok := cert.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, nil, ErrBadKey
	}

	token := &QueryToken{
		NodeID:    nodeID,
		IssuedAt:  time.Now().UnixNano(),
		TopK:      topK,
		Embedding: embedding,
	}

	digest := tokenDigest(token)
	sig, err := ecdsa.SignASN1(rand.Reader, ecKey, digest[:])
	if err != nil {
		return nil, nil, fmt.Errorf("signed memory: sign token: %w", err)
	}
	token.Signature = sig

	// Return the leaf DER cert for the receiver to verify.
	leafDER := cert.Certificate[0]
	return token, leafDER, nil
}

// tokenDigest computes the canonical SHA-256 hash that is signed/verified.
// Format: sha256(node_id_len || node_id || issued_at_be64 || top_k_be64 || embedding_f32s).
func tokenDigest(t *QueryToken) [32]byte {
	h := sha256.New()
	idBytes := []byte(t.NodeID)
	var lenBuf [8]byte
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(idBytes)))
	h.Write(lenBuf[:])
	h.Write(idBytes)
	binary.BigEndian.PutUint64(lenBuf[:], uint64(t.IssuedAt)) // #nosec G115 -- IssuedAt is a unix nanosecond timestamp; negative values are impossible in practice
	h.Write(lenBuf[:])
	binary.BigEndian.PutUint64(lenBuf[:], uint64(t.TopK)) // #nosec G115 -- TopK is a small positive int; overflow is impossible in practice
	h.Write(lenBuf[:])
	buf := make([]byte, 4*len(t.Embedding))
	for i, v := range t.Embedding {
		binary.BigEndian.PutUint32(buf[4*i:], math.Float32bits(v))
	}
	h.Write(buf)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// MemoryServer exposes a SignedMemoryStore over a mTLS TCP connection.
// Clients must send a SignedQueryRequest (JSON); the server responds with a
// SignedQueryResponse (JSON).  The mTLS layer provides transport security and
// the query token provides data-layer access control.
type MemoryServer struct { //nolint:revive // MemoryServer is intentionally explicit to distinguish from other Server types in the package
	store    *SignedMemoryStore
	tlsCfg   *tls.Config
	listener net.Listener
	quit     chan struct{}
}

// NewMemoryServer creates a MemoryServer that uses tlsCfg for transport
// security and store for signed query authorisation.
func NewMemoryServer(store *SignedMemoryStore, tlsCfg *tls.Config) *MemoryServer {
	return &MemoryServer{
		store:  store,
		tlsCfg: tlsCfg,
		quit:   make(chan struct{}),
	}
}

// Stop signals the server to stop accepting new connections.
func (ms *MemoryServer) Stop() {
	close(ms.quit)
	if ms.listener != nil {
		_ = ms.listener.Close()
	}
}

// Serve binds to addr and handles incoming signed query requests until Stop
// is called or an unrecoverable accept error occurs.
func (ms *MemoryServer) Serve(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("memory server: bind %s: %w", addr, err)
	}
	ms.listener = tls.NewListener(ln, ms.tlsCfg)
	for {
		conn, err := ms.listener.Accept()
		if err != nil {
			select {
			case <-ms.quit:
				return nil
			default:
				continue
			}
		}
		go ms.handle(conn)
	}
}

func (ms *MemoryServer) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	var req SignedQueryRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(SignedQueryResponse{Error: "decode: " + err.Error()}) //nolint:errchkjson // best-effort error response; connection is closing
		return
	}

	results, err := ms.store.AuthorisedQuery(&req)
	resp := SignedQueryResponse{Results: results}
	if err != nil {
		resp.Error = err.Error()
	}
	_ = json.NewEncoder(conn).Encode(resp) //nolint:errchkjson // best-effort response write; float32 scores are always finite
}
