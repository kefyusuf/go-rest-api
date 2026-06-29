package store

import (
	"context"
	"time"

	"go-lang/internal/cache"
	"go-lang/internal/model"
)

// CachedUserStore wraps a UserStore with a read-through cache. Reads
// consult the cache first; misses go to the underlying store and are
// written back. Writes invalidate the cache entry.
type CachedUserStore struct {
	inner UserStore
	cache *cache.UserCache
	ttl   time.Duration
}

func NewCachedUserStore(inner UserStore, userCache *cache.UserCache, ttl time.Duration) *CachedUserStore {
	return &CachedUserStore{inner: inner, cache: userCache, ttl: ttl}
}

func (s *CachedUserStore) List() ([]model.User, error) {
	return s.inner.List()
}

func (s *CachedUserStore) GetByID(id int) (model.User, error) {
	if s.cache != nil {
		ctx := context.Background()
		if data, err := s.cache.Get(ctx, int64(id)); err == nil {
			var u model.User
			if jerr := cache.JSONDecode(data, &u); jerr == nil {
				return u, nil
			}
		}
	}

	user, err := s.inner.GetByID(id)
	if err != nil {
		return model.User{}, err
	}

	if s.cache != nil {
		data, jerr := cache.JSONEncode(struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}{ID: user.ID, Name: user.Name, Email: user.Email})
		if jerr == nil {
			_ = s.cache.SetWithTTL(context.Background(), int64(id), data, s.ttl)
		}
	}
	return user, nil
}

func (s *CachedUserStore) GetByEmail(email string) (model.User, error) {
	return s.inner.GetByEmail(email)
}

func (s *CachedUserStore) Create(input model.CreateUserRequest) (model.User, error) {
	user, err := s.inner.Create(input)
	if err != nil {
		return model.User{}, err
	}
	s.invalidate(int64(user.ID))
	return user, nil
}

func (s *CachedUserStore) Update(id int, input model.UpdateUserRequest) (model.User, error) {
	user, err := s.inner.Update(id, input)
	if err != nil {
		return model.User{}, err
	}
	s.invalidate(int64(id))
	return user, nil
}

func (s *CachedUserStore) UpdatePassword(id int, passwordHash string) error {
	if err := s.inner.UpdatePassword(id, passwordHash); err != nil {
		return err
	}
	s.invalidate(int64(id))
	return nil
}

func (s *CachedUserStore) Delete(id int) error {
	if err := s.inner.Delete(id); err != nil {
		return err
	}
	s.invalidate(int64(id))
	return nil
}

func (s *CachedUserStore) invalidate(id int64) {
	if s.cache == nil {
		return
	}
	_ = s.cache.Delete(context.Background(), id)
}
