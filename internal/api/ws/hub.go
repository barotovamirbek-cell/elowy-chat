package ws

import "sync"

// Hub — это центр который хранит все подключения
// Как коммутатор на телефонной станции
type Hub struct {
	clients map[int]*Client // userID -> клиент
	mu      sync.RWMutex
}

var GlobalHub = &Hub{
	clients: make(map[int]*Client),
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	h.clients[client.UserID] = client
	h.mu.Unlock()
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	delete(h.clients, client.UserID)
	h.mu.Unlock()
}

// Отправить сообщение конкретному пользователю
func (h *Hub) SendToUser(userID int, message []byte) {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()
	if ok {
		client.Send <- message
	}
}
