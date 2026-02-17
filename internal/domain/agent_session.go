package domain

import (
	"time"
)

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

type StoredMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
