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
	GroupID        int    `json:"group_id"`        // ← добавлено для групп
	Content        string `json:"content"`
	MediaURL       string `json:"media_url"`
	MediaType      string `json:"media_type"`
	SenderID       int    `json:"sender_id"`
	SenderUsername string `json:"sender_username"`
}

// GroupChat — модель группового чата
type GroupChat struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	AvatarURL   string    `json:"avatar_url"`
	CreatedBy   int       `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	LastMessage string    `json:"last_message"`
	MemberCount int       `json:"member_count"`
}

// GroupMember — участник группы
type GroupMember struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Role        string `json:"role"` // "admin" или "member"
}

// GroupMessage — сообщение в группе
type GroupMessage struct {
	ID           int       `json:"id"`
	GroupID      int       `json:"group_id"`
	SenderID     int       `json:"sender_id"`
	SenderName   string    `json:"sender_name"`
	SenderAvatar string    `json:"sender_avatar"`
	Content      string    `json:"content"`
	MediaURL     string    `json:"media_url"`
	MediaType    string    `json:"media_type"`
	CreatedAt    time.Time `json:"created_at"`
}
