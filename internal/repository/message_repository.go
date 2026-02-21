package repository

import (
	"database/sql"
	"your_project/internal/models"
)

type MessageRepository struct {
	DB *sql.DB
}

// Создать или найти диалог между двумя пользователями
func (r *MessageRepository) GetOrCreateConversation(userID1, userID2 int) (int, error) {
	// Ищем существующий диалог
	query := `
		SELECT c.id FROM conversations c
		JOIN conversation_members m1 ON c.id = m1.conversation_id AND m1.user_id = $1
		JOIN conversation_members m2 ON c.id = m2.conversation_id AND m2.user_id = $2
		LIMIT 1`
	var convID int
	err := r.DB.QueryRow(query, userID1, userID2).Scan(&convID)
	if err == nil {
		return convID, nil
	}

	// Создаём новый диалог
	err = r.DB.QueryRow(`INSERT INTO conversations DEFAULT VALUES RETURNING id`).Scan(&convID)
	if err != nil {
		return 0, err
	}
	_, err = r.DB.Exec(`INSERT INTO conversation_members (conversation_id, user_id) VALUES ($1, $2), ($1, $3)`, convID, userID1, userID2)
	return convID, err
}

// Сохранить сообщение
func (r *MessageRepository) SaveMessage(msg models.Message) (models.Message, error) {
	query := `
		INSERT INTO messages (conversation_id, sender_id, content)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`
	err := r.DB.QueryRow(query, msg.ConversationID, msg.SenderID, msg.Content).
		Scan(&msg.ID, &msg.CreatedAt)
	return msg, err
}

// Получить сообщения диалога
func (r *MessageRepository) GetMessages(conversationID int) ([]models.Message, error) {
	query := `
		SELECT m.id, m.conversation_id, m.sender_id, u.username, m.content, m.created_at
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		WHERE m.conversation_id = $1
		ORDER BY m.created_at ASC`
	rows, err := r.DB.Query(query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		rows.Scan(&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.SenderUsername, &msg.Content, &msg.CreatedAt)
		messages = append(messages, msg)
	}
	return messages, nil
}

// Получить список диалогов пользователя
func (r *MessageRepository) GetConversations(userID int) ([]models.Conversation, error) {
	query := `
		SELECT c.id,
			u.id as other_user_id,
			u.username as other_username,
			COALESCE((SELECT content FROM messages WHERE conversation_id = c.id ORDER BY created_at DESC LIMIT 1), '') as last_message,
			c.created_at
		FROM conversations c
		JOIN conversation_members cm ON c.id = cm.conversation_id AND cm.user_id = $1
		JOIN conversation_members cm2 ON c.id = cm2.conversation_id AND cm2.user_id != $1
		JOIN users u ON cm2.user_id = u.id
		ORDER BY c.created_at DESC`
	rows, err := r.DB.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []models.Conversation
	for rows.Next() {
		var conv models.Conversation
		rows.Scan(&conv.ID, &conv.OtherUserID, &conv.OtherUsername, &conv.LastMessage, &conv.CreatedAt)
		convs = append(convs, conv)
	}
	return convs, nil
}
