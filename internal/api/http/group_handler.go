package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"your_project/internal/pkg/database"
)

// Создать группу
func CreateGroup(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	var body struct {
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		MemberIDs []int  `json:"member_ids"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if body.Name == "" {
		http.Error(w, "Название обязательно", http.StatusBadRequest)
		return
	}

	var groupID int
	err = database.DB.QueryRow(
		`INSERT INTO group_chats (name, avatar_url, created_by) VALUES ($1, $2, $3) RETURNING id`,
		body.Name, body.AvatarURL, userID,
	).Scan(&groupID)
	if err != nil {
		http.Error(w, "Ошибка создания группы", http.StatusInternalServerError)
		return
	}

	// Добавляем создателя как админа
	database.DB.Exec(
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'admin')`,
		groupID, userID,
	)

	// Добавляем остальных участников
	for _, memberID := range body.MemberIDs {
		if memberID != userID {
			database.DB.Exec(
				`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'member')`,
				groupID, memberID,
			)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"group_id": groupID})
}

// Список групп пользователя
func GetGroups(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	rows, err := database.DB.Query(`
		SELECT g.id, g.name, COALESCE(g.avatar_url,''),
			COALESCE((SELECT content FROM group_messages WHERE group_id = g.id ORDER BY created_at DESC LIMIT 1), '') as last_message,
			g.created_by
		FROM group_chats g
		JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = $1
		ORDER BY g.created_at DESC`, userID)
	if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Group struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		AvatarURL   string `json:"avatar_url"`
		LastMessage string `json:"last_message"`
		CreatedBy   int    `json:"created_by"`
	}

	var groups []Group
	for rows.Next() {
		var g Group
		rows.Scan(&g.ID, &g.Name, &g.AvatarURL, &g.LastMessage, &g.CreatedBy)
		groups = append(groups, g)
	}
	if groups == nil {
		groups = []Group{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groups)
}

// Сообщения группы
func GetGroupMessages(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	groupIDStr := r.URL.Query().Get("group_id")
	groupID, _ := strconv.Atoi(groupIDStr)

	// Проверяем что пользователь в группе
	var count int
	database.DB.QueryRow(
		`SELECT COUNT(*) FROM group_members WHERE group_id=$1 AND user_id=$2`,
		groupID, userID,
	).Scan(&count)
	if count == 0 {
		http.Error(w, "Нет доступа", http.StatusForbidden)
		return
	}

	rows, err := database.DB.Query(`
		SELECT gm.id, gm.group_id, gm.sender_id, u.username,
			gm.content, COALESCE(gm.media_url,''), COALESCE(gm.media_type,''), gm.created_at
		FROM group_messages gm
		JOIN users u ON gm.sender_id = u.id
		WHERE gm.group_id = $1
		ORDER BY gm.created_at ASC`, groupID)
	if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type GroupMessage struct {
		ID             int    `json:"id"`
		GroupID        int    `json:"group_id"`
		SenderID       int    `json:"sender_id"`
		SenderUsername string `json:"sender_username"`
		Content        string `json:"content"`
		MediaURL       string `json:"media_url"`
		MediaType      string `json:"media_type"`
		CreatedAt      string `json:"created_at"`
	}

	var msgs []GroupMessage
	for rows.Next() {
		var m GroupMessage
		rows.Scan(&m.ID, &m.GroupID, &m.SenderID, &m.SenderUsername,
			&m.Content, &m.MediaURL, &m.MediaType, &m.CreatedAt)
		msgs = append(msgs, m)
	}
	if msgs == nil {
		msgs = []GroupMessage{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

// Информация о группе с участниками
func GetGroupInfo(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	groupIDStr := r.URL.Query().Get("group_id")
	groupID, _ := strconv.Atoi(groupIDStr)

	var count int
	database.DB.QueryRow(
		`SELECT COUNT(*) FROM group_members WHERE group_id=$1 AND user_id=$2`,
		groupID, userID,
	).Scan(&count)
	if count == 0 {
		http.Error(w, "Нет доступа", http.StatusForbidden)
		return
	}

	type Member struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
		Avatar   string `json:"avatar_url"`
	}

	type GroupInfo struct {
		ID        int      `json:"id"`
		Name      string   `json:"name"`
		AvatarURL string   `json:"avatar_url"`
		CreatedBy int      `json:"created_by"`
		Members   []Member `json:"members"`
	}

	var info GroupInfo
	database.DB.QueryRow(
		`SELECT id, name, COALESCE(avatar_url,''), created_by FROM group_chats WHERE id=$1`,
		groupID,
	).Scan(&info.ID, &info.Name, &info.AvatarURL, &info.CreatedBy)

	rows, _ := database.DB.Query(`
		SELECT u.id, u.username, gm.role, COALESCE(u.avatar_url,'')
		FROM group_members gm
		JOIN users u ON gm.user_id = u.id
		WHERE gm.group_id = $1`, groupID)
	defer rows.Close()
	for rows.Next() {
		var m Member
		rows.Scan(&m.ID, &m.Username, &m.Role, &m.Avatar)
		info.Members = append(info.Members, m)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// Обновить группу
func UpdateGroup(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	var body struct {
		GroupID   int    `json:"group_id"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	// Проверяем что пользователь админ
	var role string
	database.DB.QueryRow(
		`SELECT role FROM group_members WHERE group_id=$1 AND user_id=$2`,
		body.GroupID, userID,
	).Scan(&role)
	if role != "admin" {
		http.Error(w, "Нет прав", http.StatusForbidden)
		return
	}

	database.DB.Exec(
		`UPDATE group_chats SET name=$1, avatar_url=$2 WHERE id=$3`,
		body.Name, body.AvatarURL, body.GroupID,
	)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Обновлено"})
}

// Добавить участника
func AddGroupMember(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	var body struct {
		GroupID  int `json:"group_id"`
		MemberID int `json:"member_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	var role string
	database.DB.QueryRow(
		`SELECT role FROM group_members WHERE group_id=$1 AND user_id=$2`,
		body.GroupID, userID,
	).Scan(&role)
	if role != "admin" {
		http.Error(w, "Нет прав", http.StatusForbidden)
		return
	}

	database.DB.Exec(
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'member') ON CONFLICT DO NOTHING`,
		body.GroupID, body.MemberID,
	)

	w.WriteHeader(http.StatusOK)
}

// Удалить участника / покинуть группу
func RemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	var body struct {
		GroupID  int `json:"group_id"`
		MemberID int `json:"member_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	targetID := body.MemberID
	if targetID == 0 {
		targetID = userID // покинуть группу
	}

	if targetID != userID {
		var role string
		database.DB.QueryRow(
			`SELECT role FROM group_members WHERE group_id=$1 AND user_id=$2`,
			body.GroupID, userID,
		).Scan(&role)
		if role != "admin" {
			http.Error(w, "Нет прав", http.StatusForbidden)
			return
		}
	}

	database.DB.Exec(
		`DELETE FROM group_members WHERE group_id=$1 AND user_id=$2`,
		body.GroupID, targetID,
	)

	w.WriteHeader(http.StatusOK)
}
