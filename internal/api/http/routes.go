package http

import (
	"github.com/gorilla/mux"
)

func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/register", RegisterUser).Methods("POST")
	r.HandleFunc("/api/login", LoginUser).Methods("POST")
	r.HandleFunc("/api/profile", GetUserProfile).Methods("GET")
	r.HandleFunc("/api/users", GetUsers).Methods("GET")
	r.HandleFunc("/api/conversations", GetConversations).Methods("GET")
	r.HandleFunc("/api/conversations/start", StartConversation).Methods("POST")
	r.HandleFunc("/api/messages", GetMessages).Methods("GET")
	r.HandleFunc("/ws", HandleWebSocket)
     r.HandleFunc("/api/profile/update", UpdateProfile).Methods("POST")
     r.HandleFunc("/api/profile/me", GetProfile).Methods("GET")
     r.HandleFunc("/api/cloudinary/config", GetCloudinarySignature).Methods("GET")
}
