package http

import (
	"github.com/gorilla/mux"
)

func RegisterRoutes(r *mux.Router) {
	// Существующие маршруты
	r.HandleFunc("/api/register", RegisterUser).Methods("POST")
	r.HandleFunc("/api/login", LoginUser).Methods("POST")
	r.HandleFunc("/api/profile", GetUserProfile).Methods("GET")
	r.HandleFunc("/api/profile/me", GetProfile).Methods("GET")
	r.HandleFunc("/api/profile/update", UpdateProfile).Methods("POST")
	r.HandleFunc("/api/user", GetUserProfileByID).Methods("GET")
	r.HandleFunc("/api/users", GetUsers).Methods("GET")
	r.HandleFunc("/api/conversations", GetConversations).Methods("GET")
	r.HandleFunc("/api/conversations/start", StartConversation).Methods("POST")
	r.HandleFunc("/api/messages", GetMessages).Methods("GET")
	r.HandleFunc("/api/cloudinary/config", GetCloudinaryConfig).Methods("GET")
	r.HandleFunc("/ws", HandleWebSocket)

	// Группы
	r.HandleFunc("/api/groups", GetMyGroups).Methods("GET")
	r.HandleFunc("/api/groups/create", CreateGroup).Methods("POST")
	r.HandleFunc("/api/groups/messages", GetGroupMessages).Methods("GET")
	r.HandleFunc("/api/groups/members", GetGroupMembers).Methods("GET")
	r.HandleFunc("/api/groups/add-member", AddGroupMember).Methods("POST")
	r.HandleFunc("/api/groups/remove-member", RemoveGroupMember).Methods("POST")
	r.HandleFunc("/api/groups/update", UpdateGroup).Methods("POST")

	// Настройки
	r.HandleFunc("/api/settings/change-password", ChangePassword).Methods("POST")
	r.HandleFunc("/api/settings/delete-account", DeleteAccount).Methods("DELETE")
}
