package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func SendFcmNotification(toUserID int, data map[string]string) {
	var fcmToken string
	err := GlobalHub.DB.QueryRow(
		`SELECT COALESCE(fcm_token, '') FROM users WHERE id = $1`, toUserID,
	).Scan(&fcmToken)
	if err != nil || fcmToken == "" {
		return
	}

	serverKey := os.Getenv("FCM_SERVER_KEY")
	if serverKey == "" {
		log.Println("FCM_SERVER_KEY not set")
		return
	}

	payload := map[string]interface{}{
		"to":   fcmToken,
		"data": data,
		"android": map[string]interface{}{
			"priority": "high",
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(
		context.Background(),
		"POST",
		"https://fcm.googleapis.com/fcm/send",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("key=%s", serverKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("FCM send error:", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("FCM sent to user %d, status: %d", toUserID, resp.StatusCode)
}
