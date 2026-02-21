package ws

import (
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
		case "group_message":
			// Сохраняем групповое сообщение
			_, err := database.DB.Exec(`
				INSERT INTO group_messages (group_id, sender_id, content, media_url, media_type)
				VALUES ($1, $2, $3, $4, $5)`,
				wsMsg.GroupID, c.UserID, wsMsg.Content, wsMsg.MediaURL, wsMsg.MediaType,
			)
			if err != nil {
				log.Println("Ошибка сохранения группового сообщения:", err)
				continue
			}

			response := models.WSMessage{
				Type:           "group_message",
				GroupID:        wsMsg.GroupID,
				Content:        wsMsg.Content,
				MediaURL:       wsMsg.MediaURL,
				MediaType:      wsMsg.MediaType,
				SenderID:       c.UserID,
				SenderUsername: c.Username,
			}
			responseData, _ := json.Marshal(response)

			// Отправляем всем участникам группы
			rows, err := database.DB.Query(
				`SELECT user_id FROM group_members WHERE group_id = $1 AND user_id != $2`,
				wsMsg.GroupID, c.UserID,
			)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var memberID int
					rows.Scan(&memberID)
					c.Hub.SendToUser(memberID, responseData)
				}
			}
			c.Send <- responseData

		default:
			// Личное сообщение
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
		}
	}
}

func (c *Client) WritePump() {
	defer c.Conn.Close()
	for message := range c.Send {
		c.Conn.WriteMessage(websocket.TextMessage, message)
	}
}
