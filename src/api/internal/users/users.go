package users

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"api/internal/metrics"
	"api/internal/pg_gateway"
	"api/internal/redis_gateway"

	"github.com/google/uuid"
)

type UsersManager struct {
	redis *redis_gateway.Client
	pg    *pg_gateway.Client

	metrics  *metrics.Registry
	cacheTTL time.Duration
}

type User struct {
	UserID        string `json:"user_id"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Age           int    `json:"age"`
	MaritalStatus bool   `json:"marital_status"`
}

func NewUsersManager(r *redis_gateway.Client, pg *pg_gateway.Client, reg *metrics.Registry, cacheTTL time.Duration) *UsersManager {
	if cacheTTL == 0 {
		cacheTTL = 10 * time.Minute
	}
	return &UsersManager{
		redis:    r,
		pg:       pg,
		metrics:  reg,
		cacheTTL: cacheTTL,
	}
}
func (u *UsersManager) CreateUser(ctx context.Context, first, last string, age int, marital bool) (string, error) {
	userID := uuid.NewString()
	user := User{
		UserID:        userID,
		FirstName:     first,
		LastName:      last,
		Age:           age,
		MaritalStatus: marital,
	}
	dataBytes, err := json.Marshal(user)
	if err != nil {
		u.inc("users_create_total", map[string]string{"status": "marshal_error"})
		return "", fmt.Errorf("marshal user: %w", err)
	}
	jsonStr := string(dataBytes)

	
	if err := u.pg.SaveUser(ctx, userID, jsonStr); err != nil {
		u.inc("users_create_total", map[string]string{"status": "pg_error"})
		return "", err
	}
	if u.redis != nil {
		if err := u.redis.Set(ctx, "user:"+userID, jsonStr, u.cacheTTL); err != nil {
			u.inc("users_cache_set_total", map[string]string{"status": "error"})
		} else {
			u.inc("users_cache_set_total", map[string]string{"status": "success"})
		}
	}
	u.inc("users_create_total", map[string]string{"status": "success"})
	return userID, nil
}
func (u *UsersManager) GetUsers(ctx context.Context) ([]User, error) {
	dbUsers, err := u.pg.GetUsers(ctx)
	if err != nil {
		u.inc("users_get_total", map[string]string{"status": "pg_error"})
		return nil, err
	}
	out := make([]User, 0, len(dbUsers))
	unmarshalErrors := 0
	for _, su := range dbUsers {
		var usr User
		if err := json.Unmarshal([]byte(su.Data), &usr); err != nil {
			unmarshalErrors++
			continue
		}
		out = append(out, usr)
	}
	if unmarshalErrors > 0 {
		u.inc("users_unmarshal_error_total", map[string]string{"count": fmt.Sprintf("%d", unmarshalErrors)})
	}
	u.inc("users_get_total", map[string]string{"status": "success"})
	return out, nil
}
func (u *UsersManager) inc(name string, labels map[string]string) {
	if u.metrics == nil {
		return
	}
	u.metrics.IncrementCounter(name, labels)
}
