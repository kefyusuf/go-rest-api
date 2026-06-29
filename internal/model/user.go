package model

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
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
