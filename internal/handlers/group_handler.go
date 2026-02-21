package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"golang.org/x/crypto/bcrypt"
	"your_project/internal/middleware"
)

// ========== ГРУППЫ ==========

// POST /api/groups/create
func CreateGroup(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		var req struct {
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			http.Error(w, `{"error":"Название группы обязательно"}`, http.StatusBadRequest)
			return
		}

		var groupID int
		err := db.QueryRow(`
			INSERT INTO group_chats (name, avatar_url, created_by)
			VALUES ($1, $2, $3) RETURNING id`,
			req.Name, req.AvatarURL, userID,
		).Scan(&groupID)
		if err != nil {
			http.Error(w, `{"error":"Не удалось создать группу"}`, http.StatusInternalServerError)
			return
		}

		_, err = db.Exec(`
			INSERT INTO group_members (group_id, user_id, role)
			VALUES ($1, $2, 'admin')`,
			groupID, userID,
		)
		if err != nil {
			http.Error(w, `{"error":"Не удалось добавить участника"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"group_id": groupID,
			"name":     req.Name,
		})
	}
}

// GET /api/groups
func GetMyGroups(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)

		rows, err := db.Query(`
			SELECT g.id, g.name, g.avatar_url, g.created_by,
				(SELECT content FROM group_messages gm WHERE gm.group_id = g.id ORDER BY gm.created_at DESC LIMIT 1) as last_message,
				(SELECT COUNT(*) FROM group_members gm2 WHERE gm2.group_id = g.id) as member_count
			FROM group_chats g
			JOIN group_members gm ON gm.group_id = g.id
			WHERE gm.user_id = $1
			ORDER BY g.id DESC`,
			userID,
		)
		if err != nil {
			http.Error(w, `{"error":"Ошибка получения групп"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var groups []map[string]interface{}
		for rows.Next() {
			var id, createdBy, memberCount int
			var name string
			var avatarURL, lastMsg sql.NullString
			rows.Scan(&id, &name, &avatarURL, &createdBy, &lastMsg, &memberCount)
			groups = append(groups, map[string]interface{}{
				"id":           id,
				"name":         name,
				"avatar_url":   avatarURL.String,
				"created_by":   createdBy,
				"last_message": lastMsg.String,
				"member_count": memberCount,
			})
		}
		if groups == nil {
			groups = []map[string]interface{}{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(groups)
	}
}

// GET /api/groups/messages?group_id=X
func GetGroupMessages(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		groupID, err := strconv.Atoi(r.URL.Query().Get("group_id"))
		if err != nil {
			http.Error(w, `{"error":"Неверный ID группы"}`, http.StatusBadRequest)
			return
		}

		var exists bool
		db.QueryRow(`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id=$1 AND user_id=$2)`,
			groupID, userID).Scan(&exists)
		if !exists {
			http.Error(w, `{"error":"Нет доступа"}`, http.StatusForbidden)
			return
		}

		rows, err := db.Query(`
			SELECT gm.id, gm.sender_id, u.username,
				COALESCE(u.display_name, ''), COALESCE(u.avatar_url, ''),
				gm.content, COALESCE(gm.media_url,''), COALESCE(gm.media_type,''), gm.created_at
			FROM group_messages gm
			JOIN users u ON u.id = gm.sender_id
			WHERE gm.group_id = $1
			ORDER BY gm.created_at ASC`,
			groupID,
		)
		if err != nil {
			http.Error(w, `{"error":"Ошибка получения сообщений"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var messages []map[string]interface{}
		for rows.Next() {
			var id, senderID int
			var username, displayName, avatarURL, content, mediaURL, mediaType, createdAt string
			rows.Scan(&id, &senderID, &username, &displayName, &avatarURL, &content, &mediaURL, &mediaType, &createdAt)
			name := username
			if displayName != "" {
				name = displayName
			}
			messages = append(messages, map[string]interface{}{
				"id":              id,
				"sender_id":       senderID,
				"sender_username": username,
				"sender_name":     name,
				"sender_avatar":   avatarURL,
				"content":         content,
				"media_url":       mediaURL,
				"media_type":      mediaType,
				"created_at":      createdAt,
			})
		}
		if messages == nil {
			messages = []map[string]interface{}{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	}
}

// GET /api/groups/members?group_id=X
func GetGroupMembers(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		groupID, err := strconv.Atoi(r.URL.Query().Get("group_id"))
		if err != nil {
			http.Error(w, `{"error":"Неверный ID группы"}`, http.StatusBadRequest)
			return
		}

		var exists bool
		db.QueryRow(`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id=$1 AND user_id=$2)`,
			groupID, userID).Scan(&exists)
		if !exists {
			http.Error(w, `{"error":"Нет доступа"}`, http.StatusForbidden)
			return
		}

		rows, err := db.Query(`
			SELECT u.id, u.username, COALESCE(u.display_name,''), COALESCE(u.avatar_url,''), gm.role
			FROM group_members gm
			JOIN users u ON u.id = gm.user_id
			WHERE gm.group_id = $1
			ORDER BY gm.role DESC, u.username ASC`,
			groupID,
		)
		if err != nil {
			http.Error(w, `{"error":"Ошибка"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var members []map[string]interface{}
		for rows.Next() {
			var id int
			var username, displayName, avatarURL, role string
			rows.Scan(&id, &username, &displayName, &avatarURL, &role)
			name := username
			if displayName != "" {
				name = displayName
			}
			members = append(members, map[string]interface{}{
				"id":           id,
				"username":     username,
				"display_name": name,
				"avatar_url":   avatarURL,
				"role":         role,
			})
		}
		if members == nil {
			members = []map[string]interface{}{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(members)
	}
}

// POST /api/groups/add-member
func AddGroupMember(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		var req struct {
			GroupID    int `json:"group_id"`
			TargetUser int `json:"user_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		var role string
		db.QueryRow(`SELECT role FROM group_members WHERE group_id=$1 AND user_id=$2`,
			req.GroupID, userID).Scan(&role)
		if role != "admin" {
			http.Error(w, `{"error":"Только админ может добавлять участников"}`, http.StatusForbidden)
			return
		}

		_, err := db.Exec(`
			INSERT INTO group_members (group_id, user_id, role)
			VALUES ($1, $2, 'member') ON CONFLICT DO NOTHING`,
			req.GroupID, req.TargetUser,
		)
		if err != nil {
			http.Error(w, `{"error":"Не удалось добавить"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	}
}

// POST /api/groups/remove-member
func RemoveGroupMember(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		var req struct {
			GroupID    int `json:"group_id"`
			TargetUser int `json:"user_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		var role string
		db.QueryRow(`SELECT role FROM group_members WHERE group_id=$1 AND user_id=$2`,
			req.GroupID, userID).Scan(&role)

		if req.TargetUser != userID && role != "admin" {
			http.Error(w, `{"error":"Нет прав"}`, http.StatusForbidden)
			return
		}
		db.Exec(`DELETE FROM group_members WHERE group_id=$1 AND user_id=$2`,
			req.GroupID, req.TargetUser)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	}
}

// POST /api/groups/update
func UpdateGroup(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		var req struct {
			GroupID   int    `json:"group_id"`
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		var role string
		db.QueryRow(`SELECT role FROM group_members WHERE group_id=$1 AND user_id=$2`,
			req.GroupID, userID).Scan(&role)
		if role != "admin" {
			http.Error(w, `{"error":"Только админ может редактировать группу"}`, http.StatusForbidden)
			return
		}
		db.Exec(`UPDATE group_chats SET name=$1, avatar_url=$2 WHERE id=$3`,
			req.Name, req.AvatarURL, req.GroupID)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	}
}

// ========== НАСТРОЙКИ ==========

// POST /api/settings/change-password
func ChangePassword(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		var req struct {
			OldPassword string `json:"old_password"`
			NewPassword string `json:"new_password"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.NewPassword) < 6 {
			http.Error(w, `{"error":"Пароль должен быть не менее 6 символов"}`, http.StatusBadRequest)
			return
		}

		var hashedPassword string
		err := db.QueryRow(`SELECT password_hash FROM users WHERE id=$1`, userID).Scan(&hashedPassword)
		if err != nil {
			http.Error(w, `{"error":"Пользователь не найден"}`, http.StatusNotFound)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.OldPassword)); err != nil {
			http.Error(w, `{"error":"Неверный текущий пароль"}`, http.StatusUnauthorized)
			return
		}

		newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, `{"error":"Ошибка хэширования"}`, http.StatusInternalServerError)
			return
		}
		db.Exec(`UPDATE users SET password_hash=$1 WHERE id=$2`, string(newHash), userID)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	}
}

// DELETE /api/settings/delete-account
func DeleteAccount(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		var req struct {
			Password string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		var hashedPassword string
		db.QueryRow(`SELECT password_hash FROM users WHERE id=$1`, userID).Scan(&hashedPassword)

		if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
			http.Error(w, `{"error":"Неверный пароль"}`, http.StatusUnauthorized)
			return
		}

		db.Exec(`DELETE FROM group_members WHERE user_id=$1`, userID)
		db.Exec(`DELETE FROM messages WHERE sender_id=$1`, userID)
		db.Exec(`DELETE FROM users WHERE id=$1`, userID)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	}
}
