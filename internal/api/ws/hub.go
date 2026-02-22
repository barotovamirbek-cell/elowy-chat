package ws

import (
	"database/sql"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	Clients map[int]*Client
	mu      sync.RWMutex
	DB      *sql.DB
}

// GlobalHub — глобальный экземпляр, инициализируется в main.go через InitHub
var GlobalHub *Hub

// InitHub — вызывается из main.go один раз при старте
func InitHub(db *sql.DB) {
	GlobalHub = &Hub{
		Clients: make(map[int]*Client),
		DB:      db,
	}
}

// NewHub — если нужно создать отдельный хаб
func NewHub(db *sql.DB) *Hub {
	return &Hub{
		Clients: make(map[int]*Client),
		DB:      db,
	}
}

// NewClientWithConn — создаёт клиента с websocket соединением
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

// NewClient — алиас для обратной совместимости
func NewClient(hub *Hub, conn *websocket.Conn, userID int, username string) *Client {
	return NewClientWithConn(hub, conn, userID, username)
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Clients[client.UserID] = client
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.Clients, client.UserID)
}

func (h *Hub) SendToUser(userID int, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if client, ok := h.Clients[userID]; ok {
		select {
		case client.Send <- data:
		default:
		}
	}
}

func (h *Hub) SendToGroupMembers(groupID int, excludeUserID int, data []byte) {
	rows, err := h.DB.Query(
		`SELECT user_id FROM group_members WHERE group_id = $1 AND user_id != $2`,
		groupID, excludeUserID,
	)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var uid int
		rows.Scan(&uid)
		h.SendToUser(uid, data)
	}
}
