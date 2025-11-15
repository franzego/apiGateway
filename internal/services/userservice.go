package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/franzego/apigateway/pkg/circuitbreaker"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
)

type UserServicer interface {
	ValidateUser(ctx context.Context, userID string) (bool, error)
}

type UserService struct {
	cb         *gobreaker.CircuitBreaker
	cache      *redis.Client
	baseUrl    string
	httpClient *http.Client
}

func NewUserService(baseUrl string, cache *redis.Client) *UserService {
	return &UserService{
		cb:      circuitbreaker.CircuitBreaker("email-breaker"),
		cache:   cache,
		baseUrl: baseUrl,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (u *UserService) ValidateUser(ctx context.Context, userID string) (bool, error) {
	cacheKey := fmt.Sprintf("user:%s", userID)
	cached, err := u.cache.Get(ctx, cacheKey).Result()
	if err == nil && cached == "true" {
		return true, nil
	}
	result, err := u.cb.Execute(func() (interface{}, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/users/%s", u.baseUrl, userID), nil)
		if err != nil {
			return false, err
		}
		resp, err := u.httpClient.Do(req)
		if err != nil {
			return false, nil
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return true, nil
		}
		return false, fmt.Errorf("user not found")
	})
	valid := result.(bool)
	if valid {
		u.cache.Set(ctx, fmt.Sprintf("user:%s", userID), true, 24*time.Hour)
	}
	return valid, nil
}
