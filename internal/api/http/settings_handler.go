package http

import (
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"
	"your_project/internal/pkg/database"
)

// POST /api/settings/change-password
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, `{"error":"Не авторизован"}`, http.StatusUnauthorized)
		return
	}

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
	err = database.DB.QueryRow(`SELECT password_hash FROM users WHERE id=$1`, userID).Scan(&hashedPassword)
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
		http.Error(w, `{"error":"Ошибка"}`, http.StatusInternalServerError)
		return
	}

	database.DB.Exec(`UPDATE users SET password_hash=$1 WHERE id=$2`, string(newHash), userID)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true}`))
}

// DELETE /api/settings/delete-account
func DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, `{"error":"Не авторизован"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var hashedPassword string
	database.DB.QueryRow(`SELECT password_hash FROM users WHERE id=$1`, userID).Scan(&hashedPassword)

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		http.Error(w, `{"error":"Неверный пароль"}`, http.StatusUnauthorized)
		return
	}

	// Каскадно удаляем всё
	database.DB.Exec(`DELETE FROM group_members WHERE user_id=$1`, userID)
	database.DB.Exec(`DELETE FROM messages WHERE sender_id=$1`, userID)
	database.DB.Exec(`DELETE FROM group_messages WHERE sender_id=$1`, userID)
	database.DB.Exec(`DELETE FROM users WHERE id=$1`, userID)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true}`))
}
