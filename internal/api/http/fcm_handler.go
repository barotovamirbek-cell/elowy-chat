package http

import (
	"encoding/json"
	"net/http"

	"your_project/internal/pkg/database"
)

// POST /api/fcm/token — сохраняем FCM токен пользователя
func SaveFcmToken(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		FcmToken string `json:"fcm_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FcmToken == "" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	_, err = database.DB.Exec(
		`UPDATE users SET fcm_token = $1 WHERE id = $2`,
		req.FcmToken, userID,
	)
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
