package redis

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	servicePrefix    = "cavi_service."
	jwtBlacklistKey  = servicePrefix + "jwt_blacklist."
	refreshTokenKey  = servicePrefix + "refresh."
)

type Client struct {
	rdb *redis.Client
}

func NewRedisClient() (*Client, error) {
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisDB := 0

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return &Client{rdb: rdb}, nil
}

func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.rdb.Set(ctx, key, value, expiration).Err()
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

func (c *Client) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	result, err := c.rdb.Exists(ctx, key).Result()
	return result > 0, err
}

func (c *Client) GetAllKeys(ctx context.Context, pattern string) ([]string, error) {
	return c.rdb.Keys(ctx, pattern).Result()
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

// добавляет JWT токен в blacklist на время его жизни
func (c *Client) WriteJWTToBlacklist(ctx context.Context, jwtStr string, jwtTTL time.Duration) error {
	key := jwtBlacklistKey + jwtStr
	return c.rdb.Set(ctx, key, "blacklisted", jwtTTL).Err()
}

// проверяет, находится ли JWT в blacklist
func (c *Client) CheckJWTInBlacklist(ctx context.Context, jwtStr string) (bool, error) {
	key := jwtBlacklistKey + jwtStr
	result, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// сохраняет refresh токен для пользователя
func (c *Client) SetRefreshToken(ctx context.Context, username string, token string, ttl time.Duration) error {
	key := refreshTokenKey + username
	return c.rdb.Set(ctx, key, token, ttl).Err()
}

// получает refresh токен пользователя
func (c *Client) GetRefreshToken(ctx context.Context, username string) (string, error) {
	key := refreshTokenKey + username
	return c.rdb.Get(ctx, key).Result()
}

// удаляет refresh токен пользователя
func (c *Client) DeleteRefreshToken(ctx context.Context, username string) error {
	key := refreshTokenKey + username
	return c.rdb.Del(ctx, key).Err()
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
