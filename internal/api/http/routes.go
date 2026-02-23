package http

import (
	"github.com/gorilla/mux"
)

func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/register", RegisterUser).Methods("POST")
	r.HandleFunc("/api/login", LoginUser).Methods("POST")
	r.HandleFunc("/api/profile", GetUserProfile).Methods("GET")
	r.HandleFunc("/api/profile/me", GetProfile).Methods("GET")
	r.HandleFunc("/api/profile/update", UpdateProfile).Methods("POST")
	r.HandleFunc("/api/user", GetUserProfileByID).Methods("GET")
	r.HandleFunc("/api/users", GetUsers).Methods("GET")
	r.HandleFunc("/api/conversations", GetConversations).Methods("GET")
	r.HandleFunc("/api/conversations/start", StartConversation).Methods("POST")
	r.HandleFunc("/api/conversations/delete", DeleteConversation).Methods("DELETE")
	r.HandleFunc("/api/messages", GetMessages).Methods("GET")
	r.HandleFunc("/api/cloudinary/config", GetCloudinaryConfig).Methods("GET")

	// Блокировка
	r.HandleFunc("/api/users/block", BlockUser).Methods("POST")
	r.HandleFunc("/api/users/unblock", UnblockUser).Methods("POST")
	r.HandleFunc("/api/users/blocked", GetBlockedUsers).Methods("GET")

	// Группы
	r.HandleFunc("/api/groups", GetGroups).Methods("GET")
	r.HandleFunc("/api/groups/create", CreateGroup).Methods("POST")
	r.HandleFunc("/api/groups/messages", GetGroupMessages).Methods("GET")
	r.HandleFunc("/api/groups/info", GetGroupInfo).Methods("GET")
	r.HandleFunc("/api/groups/update", UpdateGroup).Methods("POST")
	r.HandleFunc("/api/groups/members/add", AddGroupMember).Methods("POST")
	r.HandleFunc("/api/groups/members/remove", RemoveGroupMember).Methods("POST")

	r.HandleFunc("/ws", HandleWebSocket)
}
