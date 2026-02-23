package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"your_project/internal/pkg/database"
)

// DELETE /api/conversations/delete?conversation_id=X
func DeleteConversation(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	convIDStr := r.URL.Query().Get("conversation_id")
	convID, err := strconv.Atoi(convIDStr)
	if err != nil {
		http.Error(w, "Invalid conversation_id", http.StatusBadRequest)
		return
	}
	// Удаляем только если пользователь участник
	_, err = database.DB.Exec(
		`DELETE FROM messages WHERE conversation_id = $1 AND conversation_id IN (
			SELECT conversation_id FROM conversation_members WHERE user_id = $2
		)`, convID, userID,
	)
	if err != nil {
		http.Error(w, "Ошибка удаления", http.StatusInternalServerError)
		return
	}
	database.DB.Exec(`DELETE FROM conversation_members WHERE conversation_id = $1 AND user_id = $2`, convID, userID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Чат удалён"})
}

// POST /api/users/block
func BlockUser(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		BlockedUserID int `json:"blocked_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BlockedUserID == 0 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	database.DB.Exec(
		`INSERT INTO blocked_users (user_id, blocked_user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, req.BlockedUserID,
	)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Пользователь заблокирован"})
}

// POST /api/users/unblock
func UnblockUser(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		BlockedUserID int `json:"blocked_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BlockedUserID == 0 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	database.DB.Exec(
		`DELETE FROM blocked_users WHERE user_id = $1 AND blocked_user_id = $2`,
		userID, req.BlockedUserID,
	)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Пользователь разблокирован"})
}

// GET /api/users/blocked
func GetBlockedUsers(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	rows, err := database.DB.Query(
		`SELECT u.id, u.username, u.display_name, u.avatar_url
		 FROM blocked_users b JOIN users u ON b.blocked_user_id = u.id
		 WHERE b.user_id = $1`, userID,
	)
	if err != nil {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	defer rows.Close()
	var users []map[string]interface{}
	for rows.Next() {
		var id int
		var username, displayName, avatarURL string
		rows.Scan(&id, &username, &displayName, &avatarURL)
		users = append(users, map[string]interface{}{
			"id": id, "username": username,
			"display_name": displayName, "avatar_url": avatarURL,
		})
	}
	if users == nil {
		users = []map[string]interface{}{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}
