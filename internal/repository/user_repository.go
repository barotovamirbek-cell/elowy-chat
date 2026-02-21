package repository

import (
	"database/sql"
	"your_project/internal/models"
)

type UserRepository struct {
	DB *sql.DB
}

func (r *UserRepository) CreateUser(user models.User) error {
	query := `INSERT INTO users (username, password, display_name, user_tag) VALUES ($1, $2, $3, $4)`
	_, err := r.DB.Exec(query, user.Username, user.Password, user.DisplayName, user.UserTag)
	return err
}

func (r *UserRepository) GetUserByUsername(username string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, username, password, COALESCE(display_name,''), COALESCE(user_tag,'') FROM users WHERE username=$1`
	err := r.DB.QueryRow(query, username).Scan(&user.ID, &user.Username, &user.Password, &user.DisplayName, &user.UserTag)
	return user, err
}

func (r *UserRepository) SearchByTag(tag string, currentUserID int) ([]models.User, error) {
	query := `SELECT id, username, COALESCE(display_name,''), COALESCE(user_tag,'') FROM users WHERE user_tag ILIKE $1 AND id != $2 LIMIT 20`
	rows, err := r.DB.Query(query, "%"+tag+"%", currentUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User
		rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.UserTag)
		users = append(users, u)
	}
	return users, nil
}