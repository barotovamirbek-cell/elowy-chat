package ws

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"
	"your_project/internal/models"

	"github.com/gorilla/websocket"
)

type Client struct {
	UserID   int
	Username string
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *Hub
	DB       *sql.DB
}

func NewClientWithConn(hub *Hub, conn *websocket.Conn, userID int, username string) *Client {
	return &Client{
		UserID:   userID,
		Username: username,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      hub,
		DB:       hub.DB,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(1024 * 1024)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var msg models.WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "message":
			c.handlePersonalMessage(msg)
		case "group_message":
			c.handleGroupMessage(msg)
		case "call_offer", "call_answer", "call_reject", "call_end", "ice_candidate":
			var signal SignalMessage
			json.Unmarshal(message, &signal)
			signal.From = c.UserID
			signal.CallerName = c.Username
			HandleSignaling(c.Hub, c, signal)
		}
	}
}

func (c *Client) handlePersonalMessage(msg models.WSMessage) {
	var senderUsername string
	c.DB.QueryRow(`SELECT username FROM users WHERE id=$1`, c.UserID).Scan(&senderUsername)
	_, err := c.DB.Exec(
		`INSERT INTO messages (conversation_id, sender_id, content, media_url, media_type) VALUES ($1, $2, $3, $4, $5)`,
		msg.ConversationID, c.UserID, msg.Content, msg.MediaURL, msg.MediaType,
	)
	if err != nil {
		log.Println("Ошибка сохранения сообщения:", err)
		return
	}
	response := models.WSMessage{
		Type: "message", ConversationID: msg.ConversationID,
		Content: msg.Content, MediaURL: msg.MediaURL, MediaType: msg.MediaType,
		SenderID: c.UserID, SenderUsername: senderUsername,
	}
	data, _ := json.Marshal(response)
	rows, err := c.DB.Query(`SELECT user_id FROM conversation_members WHERE conversation_id=$1`, msg.ConversationID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var uid int
		rows.Scan(&uid)
		c.Hub.SendToUser(uid, data)
		// FCM если пользователь оффлайн
		if uid != c.UserID {
			c.Hub.mu.RLock()
			_, online := c.Hub.Clients[uid]
			c.Hub.mu.RUnlock()
			if !online {
				content := msg.Content
				if content == "" {
					content = "📎 Медиафайл"
				}
				SendFcmNotification(uid, map[string]string{
					"type":    "message",
					"sender":  senderUsername,
					"content": content,
					"id":      "1",
				})
			}
		}
	}
}

func (c *Client) handleGroupMessage(msg models.WSMessage) {
	var senderUsername string
	c.DB.QueryRow(`SELECT username FROM users WHERE id=$1`, c.UserID).Scan(&senderUsername)
	_, err := c.DB.Exec(
		`INSERT INTO group_messages (group_id, sender_id, content, media_url, media_type) VALUES ($1, $2, $3, $4, $5)`,
		msg.GroupID, c.UserID, msg.Content, msg.MediaURL, msg.MediaType,
	)
	if err != nil {
		log.Println("Ошибка сохранения группового сообщения:", err)
		return
	}
	response := models.WSMessage{
		Type: "group_message", GroupID: msg.GroupID,
		Content: msg.Content, MediaURL: msg.MediaURL, MediaType: msg.MediaType,
		SenderID: c.UserID, SenderUsername: senderUsername,
	}
	data, _ := json.Marshal(response)
	c.Hub.SendToGroupMembers(msg.GroupID, -1, data)
	// FCM оффлайн участникам группы
	rows2, err2 := c.DB.Query(`SELECT user_id FROM group_members WHERE group_id=$1 AND user_id!=$2`, msg.GroupID, c.UserID)
	if err2 == nil {
		defer rows2.Close()
		for rows2.Next() {
			var uid int
			rows2.Scan(&uid)
			c.Hub.mu.RLock()
			_, online := c.Hub.Clients[uid]
			c.Hub.mu.RUnlock()
			if !online {
				content := msg.Content
				if content == "" {
					content = "📎 Медиафайл"
				}
				SendFcmNotification(uid, map[string]string{
					"type":       "group_message",
					"sender":     senderUsername,
					"content":    content,
					"group_name": "Группа",
					"id":         "2",
				})
			}
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.WriteMessage(websocket.TextMessage, message)
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
