package users

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"api/internal/metrics"
	"api/internal/pg_gateway"
	"api/internal/redis_gateway"
)

type UsersManager struct {
	redis   *redis_gateway.Client
	pg      *pg_gateway.Client
	metrics *metrics.Registry
}

type User struct {
	UserID        string `json:"user_id"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Age           int    `json:"age"`
	MaritalStatus bool   `json:"marital_status"`
}

func NewUsersManager(r *redis_gateway.Client, pg *pg_gateway.Client, reg *metrics.Registry) *UsersManager {
	return &UsersManager{
		redis:   r,
		pg:      pg,
		metrics: reg,
	}
}

func (u *UsersManager) CreateUser(first, last string, age int, marital bool) (string, error) {
	userID := fmt.Sprintf("%d-%d", time.Now().UnixNano(), age)

	user := User{
		UserID:        userID,
		FirstName:     first,
		LastName:      last,
		Age:           age,
		MaritalStatus: marital,
	}

	dataBytes, err := json.Marshal(user)
	if err != nil {
		return "", err
	}
	jsonStr := string(dataBytes)

	// Save to Postgres
	if err := u.pg.SaveUser(userID, jsonStr); err != nil {
		return "", err
	}

	// Cache in Redis
	if err := u.redis.Set("user:"+userID, jsonStr); err != nil {
		log.Printf("[USERS] WARNING: failed to cache user in Redis: %v", err)
	}

	if u.metrics != nil {
		u.metrics.IncrementCounter("users_created_total", map[string]string{})
	}

	return userID, nil
}

func (u *UsersManager) GetUsers() ([]User, error) {
	dbUsers, err := u.pg.GetUsers()
	if err != nil {
		return nil, err
	}

	var out []User
	for _, su := range dbUsers {
		var usr User
		if err := json.Unmarshal([]byte(su.Data), &usr); err != nil {
			log.Printf("[USERS] ERROR unmarshalling user %s: %v", su.UserID, err)
			continue
		}
		out = append(out, usr)
	}

	return out, nil
}

