package http

import (
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"your_project/internal/pkg/database"
)

// ChangePassword — смена пароля (old_password, new_password)
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	var body struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Неверный формат", http.StatusBadRequest)
		return
	}
	if body.NewPassword == "" || body.OldPassword == "" {
		http.Error(w, "Оба пароля обязательны", http.StatusBadRequest)
		return
	}

	var currentHash string
	err = database.DB.QueryRow(`SELECT password FROM users WHERE id=$1`, userID).Scan(&currentHash)
	if err != nil {
		http.Error(w, "Пользователь не найден", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(body.OldPassword)); err != nil {
		http.Error(w, "Старый пароль неверен", http.StatusUnauthorized)
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Ошибка хеширования", http.StatusInternalServerError)
		return
	}

	_, err = database.DB.Exec(`UPDATE users SET password=$1 WHERE id=$2`, string(newHash), userID)
	if err != nil {
		http.Error(w, "Ошибка обновления пароля", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Пароль изменён"})
}

// DeleteAccount — удаление аккаунта (очищает связанные таблицы)
func DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	// Простая последовательность удаления связанных сущностей — адаптируйте под вашу схему
	tx, err := database.DB.Begin()
	if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}

	// Удаляем из group_members
	_, _ = tx.Exec(`DELETE FROM group_members WHERE user_id=$1`, userID)
	// Удаляем личные сообщения (отправленные) — или пометить как анонимные, в зависимости от логики
	_, _ = tx.Exec(`DELETE FROM messages WHERE sender_id=$1`, userID)
	// Удаляем групповые сообщения, если нужно
	_, _ = tx.Exec(`DELETE FROM group_messages WHERE sender_id=$1`, userID)
	// Удаляем из conversation_members
	_, _ = tx.Exec(`DELETE FROM conversation_members WHERE user_id=$1`, userID)
	// Наконец — сам пользователь
	_, err = tx.Exec(`DELETE FROM users WHERE id=$1`, userID)
	if err != nil {
		tx.Rollback()
		http.Error(w, "Ошибка удаления аккаунта", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Аккаунт удалён"})
}