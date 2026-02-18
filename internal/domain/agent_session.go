package domain

import (
	"time"
)

// AgentSession stores persisted assistant/session state for a user.
type AgentSession struct {
	UserID            string
	LastProactiveMsg  *time.Time
	AttemptCount      int
	JustSelfCorrected bool
	IsTyping          bool
	ChallengeJSON     *string
	MessagesJSON      string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// StoredMessage is a serialized chat message entry.
type StoredMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
