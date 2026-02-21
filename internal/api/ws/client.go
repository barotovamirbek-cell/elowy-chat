package ws

import (
	"database/sql"
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
	"your_project/internal/models"
	"your_project/internal/pkg/database"
	"your_project/internal/repository"
)

type Client struct {
	UserID   int
	Username string
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *Hub
}

func NewClient(hub *Hub, conn *websocket.Conn, userID int, username string) *Client {
	return &Client{
		UserID:   userID,
		Username: username,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      hub,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()

	repo := repository.MessageRepository{DB: database.DB}

	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var wsMsg models.WSMessage
		if err := json.Unmarshal(data, &wsMsg); err != nil {
			continue
		}

		switch wsMsg.Type {
		case "message":
			// Обычное личное сообщение — существующая логика
			msg := models.Message{
				ConversationID: wsMsg.ConversationID,
				SenderID:       c.UserID,
				SenderUsername: c.Username,
				Content:        wsMsg.Content,
				MediaURL:       wsMsg.MediaURL,
				MediaType:      wsMsg.MediaType,
			}
			saved, err := repo.SaveMessage(msg)
			if err != nil {
				log.Println("Ошибка сохранения сообщения:", err)
				continue
			}

			response := models.WSMessage{
				Type:           "message",
				ConversationID: saved.ConversationID,
				Content:        saved.Content,
				MediaURL:       saved.MediaURL,
				MediaType:      saved.MediaType,
				SenderID:       saved.SenderID,
				SenderUsername: saved.SenderUsername,
			}
			responseData, _ := json.Marshal(response)

			var otherUserID int
			database.DB.QueryRow(`
				SELECT user_id FROM conversation_members 
				WHERE conversation_id = $1 AND user_id != $2`,
				wsMsg.ConversationID, c.UserID).Scan(&otherUserID)

			c.Hub.SendToUser(otherUserID, responseData)
			c.Send <- responseData

		case "group_message":
			// Групповое сообщение
			if wsMsg.GroupID == 0 {
				continue
			}

			// Проверяем что отправитель состоит в группе
			var exists bool
			database.DB.QueryRow(
				`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id=$1 AND user_id=$2)`,
				wsMsg.GroupID, c.UserID,
			).Scan(&exists)
			if !exists {
				continue
			}

			// Сохраняем сообщение в БД
			var msgID int
			err := database.DB.QueryRow(`
				INSERT INTO group_messages (group_id, sender_id, content, media_url, media_type)
				VALUES ($1, $2, $3, $4, $5) RETURNING id`,
				wsMsg.GroupID, c.UserID, wsMsg.Content, wsMsg.MediaURL, wsMsg.MediaType,
			).Scan(&msgID)
			if err != nil {
				log.Println("Ошибка сохранения группового сообщения:", err)
				continue
			}

			// Получаем имя и аватар отправителя
			var senderName, senderAvatar string
			database.DB.QueryRow(
				`SELECT COALESCE(NULLIF(display_name,''), username), COALESCE(avatar_url,'') FROM users WHERE id=$1`,
				c.UserID,
			).Scan(&senderName, &senderAvatar)

			// Формируем ответ
			outMsg := map[string]interface{}{
				"type":          "group_message",
				"group_id":      wsMsg.GroupID,
				"id":            msgID,
				"sender_id":     c.UserID,
				"sender_name":   senderName,
				"sender_avatar": senderAvatar,
				"content":       wsMsg.Content,
				"media_url":     wsMsg.MediaURL,
				"media_type":    wsMsg.MediaType,
			}
			outBytes, _ := json.Marshal(outMsg)

			// Рассылаем всем участникам группы кто онлайн
			rows, err := database.DB.Query(
				`SELECT user_id FROM group_members WHERE group_id=$1`,
				wsMsg.GroupID,
			)
			if err != nil {
				continue
			}
			for rows.Next() {
				var memberID int
				rows.Scan(&memberID)
				c.Hub.SendToUser(memberID, outBytes)
			}
			rows.Close()

			// Отправляем себе тоже (для подтверждения)
			c.Send <- outBytes

		default:
			// Неизвестный тип — игнорируем
			_ = sql.ErrNoRows // просто чтобы импорт не ругался
		}
	}
}

func (c *Client) WritePump() {
	defer c.Conn.Close()
	for message := range c.Send {
		c.Conn.WriteMessage(websocket.TextMessage, message)
	}
}
