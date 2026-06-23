package store

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"go-lang/internal/model"
)

type PostgresUserStore struct {
	db *sql.DB
}

func NewPostgresUserStore(db *sql.DB) *PostgresUserStore {
	return &PostgresUserStore{db: db}
}

func (s *PostgresUserStore) List() ([]model.User, error) {
	rows, err := s.db.Query(`SELECT id, name, email FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []model.User{}
	for rows.Next() {
		var user model.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (s *PostgresUserStore) GetByID(id int) (model.User, error) {
	var user model.User

	err := s.db.QueryRow(`SELECT id, name, email FROM users WHERE id = $1`, id).Scan(&user.ID, &user.Name, &user.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, err
	}

	return user, nil
}

func (s *PostgresUserStore) Create(input model.CreateUserRequest) (model.User, error) {
	var user model.User

	err := s.db.QueryRow(
		`INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, name, email`,
		input.Name,
		input.Email,
	).Scan(&user.ID, &user.Name, &user.Email)
	if err != nil {
		if isUniqueViolation(err) {
			return model.User{}, ErrEmailAlreadyExists
		}
		return model.User{}, err
	}

	return user, nil
}

func (s *PostgresUserStore) Update(id int, input model.UpdateUserRequest) (model.User, error) {
	var user model.User

	err := s.db.QueryRow(
		`UPDATE users SET name = $1, email = $2 WHERE id = $3 RETURNING id, name, email`,
		input.Name,
		input.Email,
		id,
	).Scan(&user.ID, &user.Name, &user.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrUserNotFound
		}
		if isUniqueViolation(err) {
			return model.User{}, ErrEmailAlreadyExists
		}
		return model.User{}, err
	}

	return user, nil
}

func (s *PostgresUserStore) Delete(id int) error {
	result, err := s.db.Exec(`DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return false
}
