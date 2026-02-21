package http

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"

	ws "your_project/internal/api/ws"
	"your_project/internal/models"
	"your_project/internal/pkg/database"
	"your_project/internal/repository"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ---------- Register ----------

func RegisterUser(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат данных", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Имя пользователя и пароль обязательны", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Ошибка хеширования пароля", http.StatusInternalServerError)
		return
	}
	repo := repository.UserRepository{DB: database.DB}
	user := models.User{
    Username:    req.Username,
    Password:    string(hash),
    DisplayName: req.DisplayName,
    UserTag:     req.UserTag,
}
	if err := repo.CreateUser(user); err != nil {
		http.Error(w, "Пользователь уже существует", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Пользователь создан"})
}

// ---------- Login ----------

func LoginUser(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат данных", http.StatusBadRequest)
		return
	}
	repo := repository.UserRepository{DB: database.DB}
	user, err := repo.GetUserByUsername(req.Username)
	if err != nil {
		http.Error(w, "Неверное имя пользователя или пароль", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		http.Error(w, "Неверное имя пользователя или пароль", http.StatusUnauthorized)
		return
	}
	token, err := generateJWT(user.ID, user.Username)
	if err != nil {
		http.Error(w, "Ошибка генерации токена", http.StatusInternalServerError)
		return
	}
	user.Password = ""
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.LoginResponse{Token: token, User: *user})
}

// ---------- Profile ----------

func GetUserProfile(w http.ResponseWriter, r *http.Request) {
	userID, username, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"id": userID, "username": username})
}

// ---------- Пользователи ----------

func GetUsers(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}
	tag := r.URL.Query().Get("tag")
	repo := repository.UserRepository{DB: database.DB}
	users, err := repo.SearchByTag(tag, userID)
	if err != nil || users == nil {
		users = []models.User{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// ---------- Диалоги ----------

func GetConversations(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}
	repo := repository.MessageRepository{DB: database.DB}
	convs, err := repo.GetConversations(userID)
	if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	if convs == nil {
		convs = []models.Conversation{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(convs)
}

func StartConversation(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}
	var body struct {
		OtherUserID int `json:"other_user_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	repo := repository.MessageRepository{DB: database.DB}
	convID, err := repo.GetOrCreateConversation(userID, body.OtherUserID)
	if err != nil {
		http.Error(w, "Ошибка создания диалога", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"conversation_id": convID})
}

// ---------- Сообщения ----------

func GetMessages(w http.ResponseWriter, r *http.Request) {
	_, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}
	convIDStr := r.URL.Query().Get("conversation_id")
	convID, _ := strconv.Atoi(convIDStr)
	repo := repository.MessageRepository{DB: database.DB}
	msgs, err := repo.GetMessages(convID)
	if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []models.Message{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

// ---------- WebSocket ----------

func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "Токен обязателен", http.StatusUnauthorized)
		return
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default_secret"
	}
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "Неверный токен", http.StatusUnauthorized)
		return
	}
	claims := token.Claims.(jwt.MapClaims)
	userID := int(claims["user_id"].(float64))
	username := claims["username"].(string)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := ws.NewClient(ws.GlobalHub, conn, userID, username)
	ws.GlobalHub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}

// ---------- JWT helpers ----------

func generateJWT(userID int, username string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default_secret"
	}
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func parseJWTFromRequest(r *http.Request) (int, string, error) {
	authHeader := r.Header.Get("Authorization")
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default_secret"
	}
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return 0, "", err
	}
	claims := token.Claims.(jwt.MapClaims)
	return int(claims["user_id"].(float64)), claims["username"].(string), nil
}

func UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	var body struct {
		DisplayName string `json:"display_name"`
		Bio         string `json:"bio"`
		AvatarURL   string `json:"avatar_url"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	_, err = database.DB.Exec(
		`UPDATE users SET display_name=$1, bio=$2, avatar_url=$3 WHERE id=$4`,
		body.DisplayName, body.Bio, body.AvatarURL, userID,
	)
	if err != nil {
		http.Error(w, "Ошибка обновления", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Профиль обновлён"})
}

func GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}
	var u models.User
	database.DB.QueryRow(
		`SELECT id, username, COALESCE(display_name,''), COALESCE(user_tag,''), COALESCE(bio,''), COALESCE(avatar_url,'') FROM users WHERE id=$1`,
		userID,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.UserTag, &u.Bio, &u.AvatarURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}

func GetCloudinarySignature(w http.ResponseWriter, r *http.Request) {
	_, _, err := parseJWTFromRequest(r)
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}
	cloudName := os.Getenv("CLOUDINARY_CLOUD_NAME")
	apiKey := os.Getenv("CLOUDINARY_API_KEY")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"cloud_name": cloudName,
		"api_key":    apiKey,
		"upload_preset": "elowy_avatars",
	})
}