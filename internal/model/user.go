package model

import "time"

type User struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"-"`
	UpdatedAt    time.Time `json:"-"`
}

type CreateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ListUsersMeta struct {
	NextCursor *string `json:"nextCursor"`
	Limit      int     `json:"limit"`
}

type ListUsersResponse struct {
	Data []User        `json:"data"`
	Meta ListUsersMeta `json:"meta"`
}
