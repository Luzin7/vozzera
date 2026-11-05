package keyvalue

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	dialTimeout  = 2 * time.Second
	readTimeout  = 500 * time.Millisecond
	writeTimeout = 500 * time.Millisecond
	poolTimeout  = time.Second
	poolSize     = 5
	pipelinePool = 2
	maxRetries   = 1
	pingTimeout  = 3 * time.Second
)

func optionsFromURL(url string) (*redis.Options, error) {
	options, err := redis.ParseURL(url)
	if err != nil {
		return nil, invalidURLError(err)
	}

	options.DialTimeout = dialTimeout
	options.ReadTimeout = readTimeout
	options.WriteTimeout = writeTimeout
	options.PoolTimeout = poolTimeout
	options.PoolSize = poolSize
	options.MaxActiveConns = poolSize
	options.PipelinePoolSize = pipelinePool
	options.MaxRetries = maxRetries

	return options, nil
}

func NewClient(url string) (*redis.Client, error) {
	options, err := optionsFromURL(url)
	if err != nil {
		return nil, err
	}
	return redis.NewClient(options), nil
}

func Connect(ctx context.Context, url string) (*redis.Client, error) {
	client, err := NewClient(url)
	if err != nil {
		return nil, err
	}

	if _, err := ServerVersion(ctx, client); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}

func ServerVersion(ctx context.Context, client *redis.Client) (string, error) {
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		return "", unreachableError(err)
	}

	info, err := client.Info(pingCtx, "server").Result()
	if err != nil {
		return "", unreachableError(err)
	}
	return parseServerVersion(info), nil
}

func parseServerVersion(info string) string {
	version := ""
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(line, "valkey_version:"); ok {
			return value
		}
		if value, ok := strings.CutPrefix(line, "redis_version:"); ok {
			version = value
		}
	}
	if version == "" {
		return "unknown"
	}
	return version
}
