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

	privateKeyPEM := os.Getenv("FCM_PRIVATE_KEY")
	if privateKeyPEM == "" {
		return "", fmt.Errorf("FCM_PRIVATE_KEY not set")
	}
	privateKeyPEM = strings.ReplaceAll(privateKeyPEM, `\n`, "\n")

	rsaKey, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return "", fmt.Errorf("parse private key: %v", err)
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

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not RSA key")
	}
	return rsaKey, nil
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
