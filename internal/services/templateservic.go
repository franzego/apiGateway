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

type TemplateServicer interface {
	ValidateTemplate(ctx context.Context, tempID string) (bool, error)
}

type TemplateService struct {
	cb         *gobreaker.CircuitBreaker
	cache      *redis.Client
	httpclient *http.Client
	baseUrl    string
}

func NewTemplateService(baseUrl string, cache *redis.Client) *TemplateService {
	return &TemplateService{
		cb:    circuitbreaker.CircuitBreaker("template-breaker"),
		cache: cache,
		httpclient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseUrl: baseUrl,
	}
}

func (t *TemplateService) ValidateTemplate(ctx context.Context, tempID string) (bool, error) {
	cachedKey := fmt.Sprintf("%s/template/%s", t.baseUrl, tempID)
	found, err := t.cache.Get(ctx, cachedKey).Result()
	if err == nil || found == "true" {
		return true, nil
	}
	result, err := t.cb.Execute(func() (interface{}, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/templates/%s", t.baseUrl, tempID), nil)
		if err != nil {
			return false, fmt.Errorf("error creating request")
		}
		resp, err := t.httpclient.Do(req)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return true, nil
		}
		return false, fmt.Errorf("template: %s was not found", tempID)
	})
	valid := result.(bool)
	if valid {
		t.cache.Set(ctx, cachedKey, true, 5*time.Minute)
	}
	return valid, nil
}
