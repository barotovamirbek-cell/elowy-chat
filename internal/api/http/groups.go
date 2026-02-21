package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"your_project/internal/models"
	"your_project/internal/pkg/database"
)


// =======================
// GET MY GROUPS
// =======================
func GetMyGroups(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	rows, err := database.DB.Query(`
		SELECT g.id, g.name, COALESCE(g.description,''), COALESCE(g.avatar_url,'')
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id
		WHERE gm.user_id = $1
	`, userID)
	if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Group struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		AvatarURL   string `json:"avatar_url"`
	}

	var groups []Group

	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.AvatarURL); err == nil {
			groups = append(groups, g)
		}
	}

	if groups == nil {
		groups = []Group{}
	}

	json.NewEncoder(w).Encode(groups)
}


// =======================
// CREATE GROUP
// =======================
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
		http.Error(w, "Неверный формат", http.StatusBadRequest)
		return
	}

	if body.Name == "" {
		http.Error(w, "Название обязательно", http.StatusBadRequest)
		return
	}

	var groupID int
	err = database.DB.QueryRow(`
		INSERT INTO groups (name, description, avatar_url, created_at)
		VALUES ($1,$2,$3,NOW())
		RETURNING id
	`, body.Name, body.Description, body.AvatarURL).Scan(&groupID)

	if err != nil {
		http.Error(w, "Ошибка создания", http.StatusInternalServerError)
		return
	}

	// Добавляем владельца
	_, _ = database.DB.Exec(`
		INSERT INTO group_members (group_id,user_id,role,joined_at)
		VALUES ($1,$2,'owner',NOW())
	`, groupID, userID)

	// Добавляем остальных
	for _, id := range body.MemberIDs {
		if id == userID {
			continue
		}
		_, _ = database.DB.Exec(`
			INSERT INTO group_members (group_id,user_id,role,joined_at)
			VALUES ($1,$2,'member',NOW())
			ON CONFLICT DO NOTHING
		`, groupID, id)
	}

	json.NewEncoder(w).Encode(map[string]int{"group_id": groupID})
}


// =======================
// GET GROUP MESSAGES
// =======================
func GetGroupMessages(w http.ResponseWriter, r *http.Request) {
	_, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	groupID, _ := strconv.Atoi(r.URL.Query().Get("group_id"))
	if groupID == 0 {
		http.Error(w, "group_id обязателен", http.StatusBadRequest)
		return
	}

	rows, err := database.DB.Query(`
		SELECT gm.id, gm.group_id, gm.sender_id,
		       COALESCE(u.username,''), gm.content,
		       COALESCE(gm.media_url,''), COALESCE(gm.media_type,''), gm.created_at
		FROM group_messages gm
		LEFT JOIN users u ON u.id = gm.sender_id
		WHERE gm.group_id=$1
		ORDER BY gm.created_at ASC
	`, groupID)

	if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Msg struct {
		ID        int    `json:"id"`
		GroupID   int    `json:"group_id"`
		SenderID  int    `json:"sender_id"`
		Username  string `json:"sender_username"`
		Content   string `json:"content"`
		MediaURL  string `json:"media_url"`
		MediaType string `json:"media_type"`
		CreatedAt string `json:"created_at"`
	}

	var messages []Msg

	for rows.Next() {
		var m Msg
		if err := rows.Scan(&m.ID, &m.GroupID, &m.SenderID, &m.Username,
			&m.Content, &m.MediaURL, &m.MediaType, &m.CreatedAt); err == nil {
			messages = append(messages, m)
		}
	}

	if messages == nil {
		messages = []Msg{}
	}

	json.NewEncoder(w).Encode(messages)
}


// =======================
// GET GROUP MEMBERS
// =======================
func GetGroupMembers(w http.ResponseWriter, r *http.Request) {
	_, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	groupID, _ := strconv.Atoi(r.URL.Query().Get("group_id"))
	if groupID == 0 {
		http.Error(w, "group_id обязателен", http.StatusBadRequest)
		return
	}

	rows, err := database.DB.Query(`
		SELECT u.id, u.username, COALESCE(u.display_name,''), COALESCE(u.avatar_url,'')
		FROM users u
		JOIN group_members gm ON gm.user_id=u.id
		WHERE gm.group_id=$1
	`, groupID)

	if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []models.User

	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.AvatarURL); err == nil {
			u.Password = ""
			users = append(users, u)
		}
	}

	if users == nil {
		users = []models.User{}
	}

	json.NewEncoder(w).Encode(users)
}


// =======================
// ADD MEMBER (с проверкой роли)
// =======================
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

	var role string
	err = database.DB.QueryRow(`
		SELECT role FROM group_members
		WHERE group_id=$1 AND user_id=$2
	`, body.GroupID, userID).Scan(&role)

	if err != nil || (role != "owner" && role != "admin") {
		http.Error(w, "Нет прав", http.StatusForbidden)
		return
	}

	_, err = database.DB.Exec(`
		INSERT INTO group_members (group_id,user_id,role,joined_at)
		VALUES ($1,$2,'member',NOW())
		ON CONFLICT DO NOTHING
	`, body.GroupID, body.UserID)

	if err != nil {
		http.Error(w, "Ошибка добавления", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Добавлен"})
}

// =======================
// REMOVE MEMBER
// =======================
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

	// Проверяем роль текущего пользователя
	var role string
	err = database.DB.QueryRow(`
		SELECT role FROM group_members
		WHERE group_id=$1 AND user_id=$2
	`, body.GroupID, userID).Scan(&role)

	if err != nil || (role != "owner" && role != "admin") {
		http.Error(w, "Нет прав", http.StatusForbidden)
		return
	}

	_, err = database.DB.Exec(`
		DELETE FROM group_members
		WHERE group_id=$1 AND user_id=$2
	`, body.GroupID, body.UserID)

	if err != nil {
		http.Error(w, "Ошибка удаления", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Удалён"})
}


// =======================
// UPDATE GROUP
// =======================
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

	// Проверяем роль
	var role string
	err = database.DB.QueryRow(`
		SELECT role FROM group_members
		WHERE group_id=$1 AND user_id=$2
	`, body.GroupID, userID).Scan(&role)

	if err != nil || (role != "owner" && role != "admin") {
		http.Error(w, "Нет прав", http.StatusForbidden)
		return
	}

	_, err = database.DB.Exec(`
		UPDATE groups
		SET name=$1, description=$2, avatar_url=$3
		WHERE id=$4
	`, body.Name, body.Description, body.AvatarURL, body.GroupID)

	if err != nil {
		http.Error(w, "Ошибка обновления", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Обновлено"})
}