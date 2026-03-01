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
	"strings"
	"time"
)

const (
	fcmProjectID   = "elowy-chat"
	fcmClientEmail = "firebase-adminsdk-fbsvc@elowy-chat.iam.gserviceaccount.com"
	fcmTokenURL    = "https://oauth2.googleapis.com/token"
	fcmScope       = "https://www.googleapis.com/auth/firebase.messaging"
)

var cachedFcmToken string
var fcmTokenExpiry time.Time

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

	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"token": fcmToken,
			"data":  data,
			"android": map[string]interface{}{
				"priority": "high",
			},
		},
	}

	body, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", fcmProjectID)
	req, err := http.NewRequestWithContext(context.Background(), "POST", apiURL, bytes.NewBuffer(body))
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

	rsaKey, err := loadPrivateKey()
	if err != nil {
		return "", err
	}

	now := time.Now()
	jwt, err := buildJWT(map[string]interface{}{
		"iss":   fcmClientEmail,
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

func loadPrivateKey() (*rsa.PrivateKey, error) {
	raw := os.Getenv("FCM_PRIVATE_KEY")
	if raw == "" {
		return nil, fmt.Errorf("FCM_PRIVATE_KEY not set")
	}

	// Railway хранит \n как литерал — восстанавливаем переносы
	raw = strings.ReplaceAll(raw, `\n`, "\n")

	// Если нет заголовка PEM — оборачиваем
	if !strings.Contains(raw, "-----BEGIN") {
		raw = "-----BEGIN PRIVATE KEY-----\n" + raw + "\n-----END PRIVATE KEY-----\n"
	}

	// Убеждаемся что строки не длиннее 64 символов (требование PEM)
	raw = normalizePEM(raw)

	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		// Пробуем base64 напрямую
		return parseRawBase64Key(raw)
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("ParsePKCS8: %v", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not RSA key")
	}
	return rsaKey, nil
}

func normalizePEM(raw string) string {
	lines := strings.Split(raw, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "-----") {
			result = append(result, line)
			continue
		}
		// Разбиваем длинные строки по 64 символа
		for len(line) > 64 {
			result = append(result, line[:64])
			line = line[64:]
		}
		if len(line) > 0 {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n") + "\n"
}

func parseRawBase64Key(raw string) (*rsa.PrivateKey, error) {
	// Убираем заголовки и пробелы
	raw = strings.ReplaceAll(raw, "-----BEGIN PRIVATE KEY-----", "")
	raw = strings.ReplaceAll(raw, "-----END PRIVATE KEY-----", "")
	raw = strings.ReplaceAll(raw, "\n", "")
	raw = strings.TrimSpace(raw)

	der, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		der, err = base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("base64 decode: %v", err)
		}
	}

	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("ParsePKCS8 from raw: %v", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not RSA key")
	}
	return rsaKey, nil
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
