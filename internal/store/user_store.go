package store

import (
	"errors"
	"sync"

	"github.com/kefyusuf/go-rest-api/internal/model"
)

var ErrUserNotFound = errors.New("user not found")
var ErrEmailAlreadyExists = errors.New("email already exists")

type UserStore interface {
	// List returns at most limit users ordered by ascending id whose
	// id is strictly greater than afterID, so callers can page with
	// keyset pagination (afterID = last id of the previous page,
	// afterID 0 = first page). limit must be positive.
	List(afterID int64, limit int) ([]model.User, error)
	GetByID(id int) (model.User, error)
	GetByEmail(email string) (model.User, error)
	Create(input model.CreateUserRequest) (model.User, error)
	Update(id int, input model.UpdateUserRequest) (model.User, error)
	UpdatePassword(id int, passwordHash string) error
	Delete(id int) error
}

type MemoryUserStore struct {
	mu     sync.Mutex
	users  []model.User
	nextID int
}

func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{
		users:  []model.User{},
		nextID: 1,
	}
}

func (s *MemoryUserStore) List(afterID int64, limit int) ([]model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	users := make([]model.User, 0, limit)
	for _, user := range s.users {
		if int64(user.ID) <= afterID {
			continue
		}
		users = append(users, user)
		if len(users) == limit {
			break
		}
	}

	return users, nil
}

func (s *MemoryUserStore) GetByID(id int) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, user := range s.users {
		if user.ID == id {
			return user, nil
		}
	}

	return model.User{}, ErrUserNotFound
}

func (s *MemoryUserStore) GetByEmail(email string) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, user := range s.users {
		if user.Email == email {
			return user, nil
		}
	}

	return model.User{}, ErrUserNotFound
}

func (s *MemoryUserStore) Create(input model.CreateUserRequest) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, user := range s.users {
		if user.Email == input.Email {
			return model.User{}, ErrEmailAlreadyExists
		}
	}

	user := model.User{
		ID:           s.nextID,
		Name:         input.Name,
		Email:        input.Email,
		PasswordHash: input.Password,
	}

	s.users = append(s.users, user)
	s.nextID++

	return user, nil
}

func (s *MemoryUserStore) Update(id int, input model.UpdateUserRequest) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, user := range s.users {
		if user.ID != id && user.Email == input.Email {
			return model.User{}, ErrEmailAlreadyExists
		}

		if user.ID == id {
			s.users[i].Name = input.Name
			s.users[i].Email = input.Email
			return s.users[i], nil
		}
	}

	return model.User{}, ErrUserNotFound
}

func (s *MemoryUserStore) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, user := range s.users {
		if user.ID == id {
			s.users = append(s.users[:i], s.users[i+1:]...)
			return nil
		}
	}

	return ErrUserNotFound
}

func (s *MemoryUserStore) UpdatePassword(id int, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, user := range s.users {
		if user.ID == id {
			s.users[i].PasswordHash = passwordHash
			return nil
		}
	}
	return ErrUserNotFound
}
