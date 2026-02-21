package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"your_project/internal/models"
	"your_project/internal/pkg/database"
)

// GetMyGroups — возвращает список групп, в которых состоит пользователь
func GetMyGroups(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	rows, err := database.DB.Query(`
		SELECT g.id, g.name, COALESCE(g.description, ''), COALESCE(g.avatar_url,'')
		FROM groups g
		JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = $1
	`, userID)
	if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type GroupOut struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		AvatarURL   string `json:"avatar_url"`
	}

	var res []GroupOut
	for rows.Next() {
		var g GroupOut
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.AvatarURL); err != nil {
			continue
		}
		res = append(res, g)
	}

	if res == nil {
		res = []GroupOut{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// CreateGroup — создаёт новую группу и добавляет создателя как участника
func CreateGroup(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		AvatarURL   string `json:"avatar_url"`
		MemberIDs   []int  `json:"member_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Неверный формат данных", http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		http.Error(w, "Имя группы обязательно", http.StatusBadRequest)
		return
	}

	var groupID int
	err = database.DB.QueryRow(
		`INSERT INTO groups (name, description, avatar_url, created_at) VALUES ($1, $2, $3, NOW()) RETURNING id`,
		body.Name, body.Description, body.AvatarURL,
	).Scan(&groupID)
	if err != nil {
		http.Error(w, "Ошибка создания группы", http.StatusInternalServerError)
		return
	}

	// Добавляем создателя
	_, _ = database.DB.Exec(`INSERT INTO group_members (group_id, user_id, role, joined_at) VALUES ($1, $2, $3, NOW())`, groupID, userID, "owner")
	// Добавляем остальных членов (если есть)
	for _, mid := range body.MemberIDs {
		if mid == userID {
			continue
		}
		_, _ = database.DB.Exec(`INSERT INTO group_members (group_id, user_id, role, joined_at) VALUES ($1, $2, $3, NOW()) ON CONFLICT DO NOTHING`, groupID, mid, "member")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"group_id": groupID})
}

// GetGroupMessages — возвращает сообщения группы
func GetGroupMessages(w http.ResponseWriter, r *http.Request) {
	_, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	gidStr := r.URL.Query().Get("group_id")
	gid, _ := strconv.Atoi(gidStr)
	if gid == 0 {
		http.Error(w, "group_id обязателен", http.StatusBadRequest)
		return
	}

	rows, err := database.DB.Query(`
		SELECT gm.id, gm.group_id, gm.sender_id, COALESCE(u.username,''), gm.content, COALESCE(gm.media_url,''), COALESCE(gm.media_type,''), gm.created_at
		FROM group_messages gm
		LEFT JOIN users u ON u.id = gm.sender_id
		WHERE gm.group_id = $1
		ORDER BY gm.created_at ASC
	`, gid)
	if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type MsgOut struct {
		ID        int    `json:"id"`
		GroupID   int    `json:"group_id"`
		SenderID  int    `json:"sender_id"`
		Sender    string `json:"sender_username"`
		Content   string `json:"content"`
		MediaURL  string `json:"media_url"`
		MediaType string `json:"media_type"`
		CreatedAt string `json:"created_at"`
	}

	var res []MsgOut
	for rows.Next() {
		var m MsgOut
		if err := rows.Scan(&m.ID, &m.GroupID, &m.SenderID, &m.Sender, &m.Content, &m.MediaURL, &m.MediaType, &m.CreatedAt); err != nil {
			continue
		}
		res = append(res, m)
	}
	if res == nil {
		res = []MsgOut{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// GetGroupMembers — список участников группы
func GetGroupMembers(w http.ResponseWriter, r *http.Request) {
	_, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	gidStr := r.URL.Query().Get("group_id")
	gid, _ := strconv.Atoi(gidStr)
	if gid == 0 {
		http.Error(w, "group_id обязателен", http.StatusBadRequest)
		return
	}

	rows, err := database.DB.Query(`
		SELECT u.id, u.username, COALESCE(u.display_name,''), COALESCE(u.avatar_url,'')
		FROM users u
		JOIN group_members gm ON gm.user_id = u.id
		WHERE gm.group_id = $1
	`, gid)
	if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var res []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.AvatarURL); err != nil {
			continue
		}
		u.Password = ""
		res = append(res, u)
	}
	if res == nil {
		res = []models.User{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// AddGroupMember — добавить участника
func AddGroupMember(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	var body struct {
		GroupID int `json:"group_id"`
		UserID  int `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Неверный формат", http.StatusBadRequest)
		return
	}
	// TODO: проверить права userID (создатель/админ)
	_, err = database.DB.Exec(`INSERT INTO group_members (group_id, user_id, role, joined_at) VALUES ($1, $2, $3, NOW()) ON CONFLICT DO NOTHING`, body.GroupID, body.UserID, "member")
	if err != nil {
		http.Error(w, "Ошибка добавления", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Добавлен"})
}

// RemoveGroupMember — удалить участника
func RemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	var body struct {
		GroupID int `json:"group_id"`
		UserID  int `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Неверный формат", http.StatusBadRequest)
		return
	}

	// TODO: проверить права userID (создатель/админ)
	_, err = database.DB.Exec(`DELETE FROM group_members WHERE group_id=$1 AND user_id=$2`, body.GroupID, body.UserID)
	if err != nil {
		http.Error(w, "Ошибка удаления", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Удалён"})
}

// UpdateGroup — обновить информацию о группе
func UpdateGroup(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	var body struct {
		GroupID     int    `json:"group_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		AvatarURL   string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Неверный формат", http.StatusBadRequest)
		return
	}

	// TODO: проверить права userID (владелец/админ)
	_, err = database.DB.Exec(`UPDATE groups SET name=$1, description=$2, avatar_url=$3 WHERE id=$4`, body.Name, body.Description, body.AvatarURL, body.GroupID)
	if err != nil {
		http.Error(w, "Ошибка обновления", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Обновлено"})
}