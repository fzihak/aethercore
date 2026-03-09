package discord

import (
	"encoding/json"
	"fmt"
)

// Gateway opcodes (Discord Gateway API — https://discord.com/developers/docs/topics/opcodes-and-status-codes)
const (
	OpcodeDispatch       = 0
	OpcodeHeartbeat      = 1
	OpcodeIdentify       = 2
	OpcodeReconnect      = 7
	OpcodeInvalidSession = 9
	OpcodeHello          = 10
	OpcodeHeartbeatACK   = 11
)

// Gateway intent bitfield values.
// MESSAGE_CONTENT (1<<15) is a privileged intent and must be enabled
// in the Discord Developer Portal under Bot > Privileged Gateway Intents.
const (
	IntentGuilds         = 1 << 0  // 1
	IntentGuildMessages  = 1 << 9  // 512
	IntentDirectMessages = 1 << 12 // 4096
	IntentMessageContent = 1 << 15 // 32768 (privileged)

	// DefaultIntents covers guilds, DMs, and message content.
	DefaultIntents = IntentGuilds | IntentGuildMessages | IntentDirectMessages | IntentMessageContent
)

// User represents a Discord user object.
type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator,omitempty"`
	GlobalName    string `json:"global_name,omitempty"`
	Bot           bool   `json:"bot,omitempty"`
}

// Channel represents a Discord channel object (partial fields).
type Channel struct {
	ID   string `json:"id"`
	Type int    `json:"type"` // 0=GUILD_TEXT, 1=DM
	Name string `json:"name,omitempty"`
}

// Message represents a Discord message object.
type Message struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Author    *User  `json:"author,omitempty"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

// GatewayPayload is the JSON envelope for all Discord Gateway messages.
type GatewayPayload struct {
	Op   int             `json:"op"`
	Data json.RawMessage `json:"d"`
	Seq  *int64          `json:"s,omitempty"`
	Type string          `json:"t,omitempty"`
}

// GatewayHello is the data payload for opcode 10 (Hello).
type GatewayHello struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

// GatewayIdentify is the data payload for opcode 2 (Identify).
type GatewayIdentify struct {
	Token      string            `json:"token"`
	Intents    int               `json:"intents"`
	Properties map[string]string `json:"properties"`
}

// ReadyEvent is the data payload for the READY dispatch event.
type ReadyEvent struct {
	V    int    `json:"v"`
	User *User  `json:"user"`
}

// CreateMessageRequest is the body for the REST create-message endpoint.
type CreateMessageRequest struct {
	Content string `json:"content"`
}

// APIError represents an error response from the Discord REST API.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("discord: API error %d: %s", e.Code, e.Message)
}
