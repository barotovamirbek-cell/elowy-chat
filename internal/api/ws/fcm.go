package ws

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	fcmProjectID = "elowy-chat"
	fcmTokenURL  = "https://oauth2.googleapis.com/token"
	fcmScope     = "https://www.googleapis.com/auth/firebase.messaging"
)

var cachedFcmToken string
var fcmTokenExpiry time.Time

type serviceAccountJSON struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
}

func SendFcmNotification(toUserID int, data map[string]string) {
	var fcmToken string
	err := GlobalHub.DB.QueryRow(
		`SELECT COALESCE(fcm_token, '') FROM users WHERE id = $1`, toUserID,
	).Scan(&fcmToken)
	if err != nil || fcmToken == "" {
		return
	}

	accessToken, err := getFcmAccessToken()
	if err != nil {
		log.Printf("FCM: get access token error: %v", err)
		return
	}

	// Заголовок и тело для системного уведомления Android
	title := data["sender"]
	body := data["content"]
	if title == "" {
		title = "Новое сообщение"
	}
	if body == "" {
		body = "📎 Медиафайл"
	}
	if data["type"] == "group_message" {
		if data["group_name"] != "" {
			title = data["group_name"]
		}
		if data["sender"] != "" {
			body = data["sender"] + ": " + body
		}
	}

	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"token": fcmToken,
			// data — для Flutter обработчика
			"data": data,
			// notification — системное уведомление Android (показывается даже без Flutter)
			"notification": map[string]string{
				"title": title,
				"body":  body,
			},
			"android": map[string]interface{}{
				"priority": "high",
				"notification": map[string]interface{}{
					"channel_id":          "messages",
					"notification_priority": "PRIORITY_MAX",
					"sound":               "default",
				},
			},
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", fcmProjectID)
	req, err := http.NewRequestWithContext(context.Background(), "POST", apiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("FCM send error: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("FCM v1 sent to user %d, status: %d", toUserID, resp.StatusCode)
}

func getFcmAccessToken() (string, error) {
	if cachedFcmToken != "" && time.Now().Before(fcmTokenExpiry) {
		return cachedFcmToken, nil
	}

	saJSON := os.Getenv("FCM_SERVICE_ACCOUNT")
	if saJSON == "" {
		return "", fmt.Errorf("FCM_SERVICE_ACCOUNT not set")
	}

	var sa serviceAccountJSON
	if err := json.Unmarshal([]byte(saJSON), &sa); err != nil {
		return "", fmt.Errorf("parse service account JSON: %v", err)
	}

	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM from service account")
	}
	keyIface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse private key: %v", err)
	}
	rsaKey, ok := keyIface.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("not RSA key")
	}

	now := time.Now()
	jwt, err := buildJWT(map[string]interface{}{
		"iss":   sa.ClientEmail,
		"scope": fcmScope,
		"aud":   fcmTokenURL,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}, rsaKey)
	if err != nil {
		return "", fmt.Errorf("build JWT: %v", err)
	}

	resp, err := http.PostForm(fcmTokenURL, url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {jwt},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	token, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("no access_token: %v", result)
	}

	cachedFcmToken = token
	fcmTokenExpiry = now.Add(55 * time.Minute)
	return token, nil
}

func buildJWT(claims map[string]interface{}, key *rsa.PrivateKey) (string, error) {
	header := base64.RawURLEncoding.EncodeToString(mustMarshal(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	}))
	payload := base64.RawURLEncoding.EncodeToString(mustMarshal(claims))
	input := header + "." + payload

	h := sha256.New()
	h.Write([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h.Sum(nil))
	if err != nil {
		return "", err
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
