package ws

import (
	"encoding/json"
	"log"
	"sync"
)

type SignalMessage struct {
	Type       string      `json:"type"`
	From       int         `json:"from"`
	To         int         `json:"to"`
	GroupID    int         `json:"group_id"`
	RoomID     string      `json:"room_id"`
	SDP        string      `json:"sdp"`
	Candidate  interface{} `json:"candidate"`
	CallerName string      `json:"caller_name"`
}

type CallRoom struct {
	RoomID       string
	Participants map[int]*Client
	IsGroup      bool
	mu           sync.Mutex
}

var (
	callRooms   = make(map[string]*CallRoom)
	callRoomsMu sync.Mutex
)

func GetOrCreateRoom(roomID string, isGroup bool) *CallRoom {
	callRoomsMu.Lock()
	defer callRoomsMu.Unlock()
	if room, ok := callRooms[roomID]; ok {
		return room
	}
	room := &CallRoom{
		RoomID:       roomID,
		Participants: make(map[int]*Client),
		IsGroup:      isGroup,
	}
	callRooms[roomID] = room
	return room
}

func DeleteRoom(roomID string) {
	callRoomsMu.Lock()
	defer callRoomsMu.Unlock()
	delete(callRooms, roomID)
}

func (r *CallRoom) AddParticipant(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Participants[c.UserID] = c
}

func (r *CallRoom) RemoveParticipant(userID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Participants, userID)
}

func (r *CallRoom) Broadcast(data []byte, excludeUserID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for uid, c := range r.Participants {
		if uid != excludeUserID {
			select {
			case c.Send <- data:
			default:
				log.Printf("Не удалось отправить сигнал участнику %d", uid)
			}
		}
	}
}

func HandleSignaling(hub *Hub, client *Client, signal SignalMessage) {
	data, _ := json.Marshal(signal)

	switch signal.Type {
	case "call_offer":
		// Личный звонок
		if signal.To != 0 {
			hub.SendToUser(signal.To, data)
		}
		// Групповой звонок
		if signal.GroupID != 0 {
			room := GetOrCreateRoom(signal.RoomID, true)
			room.AddParticipant(client)
			hub.SendToGroupMembers(signal.GroupID, client.UserID, data)
		}

	case "call_answer":
		room := GetOrCreateRoom(signal.RoomID, signal.GroupID != 0)
		room.AddParticipant(client)
		// signal.To = ID звонящего (кому отправить ответ)
		// signal.From уже перезаписан на ID отвечающего в client.go
		if signal.To != 0 {
			hub.SendToUser(signal.To, data)
		}

	case "call_reject", "call_end":
		if signal.To != 0 {
			hub.SendToUser(signal.To, data)
		}
		if signal.GroupID != 0 {
			hub.SendToGroupMembers(signal.GroupID, client.UserID, data)
		}
		room := GetOrCreateRoom(signal.RoomID, false)
		room.RemoveParticipant(client.UserID)
		if len(room.Participants) == 0 {
			DeleteRoom(signal.RoomID)
		}

	case "ice_candidate":
		if signal.To != 0 {
			hub.SendToUser(signal.To, data)
		} else {
			room := GetOrCreateRoom(signal.RoomID, false)
			room.Broadcast(data, client.UserID)
		}
	}
}
