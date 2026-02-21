package models

import "time"

type Message struct {
	ID             int       `json:"id"`
	ConversationID int       `json:"conversation_id"`
	SenderID       int       `json:"sender_id"`
	SenderUsername string    `json:"sender_username"`
	Content        string    `json:"content"`
	MediaURL       string    `json:"media_url"`
	MediaType      string    `json:"media_type"`
	CreatedAt      time.Time `json:"created_at"`
}

type Conversation struct {
	ID            int       `json:"id"`
	OtherUserID   int       `json:"other_user_id"`
	OtherUsername string    `json:"other_username"`
	LastMessage   string    `json:"last_message"`
	CreatedAt     time.Time `json:"created_at"`
}

type WSMessage struct {
	Type           string `json:"type"`
	ConversationID int    `json:"conversation_id"`
	GroupID        int    `json:"group_id"`
	Content        string `json:"content"`
	MediaURL       string `json:"media_url"`
	MediaType      string `json:"media_type"`
	SenderID       int    `json:"sender_id"`
	SenderUsername string `json:"sender_username"`
}
