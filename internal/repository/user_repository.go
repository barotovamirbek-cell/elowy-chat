package repository

import (
	"database/sql"
	"your_project/internal/models"
)

type UserRepository struct {
	DB *sql.DB
}

func (r *UserRepository) CreateUser(user models.User) error {
	query := `INSERT INTO users (username, password) VALUES ($1, $2)`
	_, err := r.DB.Exec(query, user.Username, user.Password)
	return err
}

func (r *UserRepository) GetUserByUsername(username string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, username, password FROM users WHERE username=$1`

	err := r.DB.QueryRow(query, username).
		Scan(&user.ID, &user.Username, &user.Password)

	return user, err
}
