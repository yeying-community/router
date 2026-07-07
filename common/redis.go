package common

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
)

var RDB redis.Cmdable
var RedisEnabled = true

func ensureRedisClient() error {
	if !RedisEnabled {
		return errors.New("redis is disabled")
	}
	if RDB == nil {
		return errors.New("redis client is not initialized")
	}
	return nil
}

// InitRedisClient This function is called after init()
func InitRedisClient() (err error) {
	cacheType := strings.ToLower(strings.TrimSpace(config.CacheType))
	if cacheType == "" {
		cacheType = config.CacheTypeLocal
	}
	if cacheType != config.CacheTypeRedis {
		RedisEnabled = false
		logger.SysLog("cache.type=" + cacheType + ", Redis is not enabled")
		return nil
	}
	if RedisConnString == "" {
		RedisEnabled = false
		return errors.New("cache.type=redis requires redis.conn_string")
	}
	redisConnString := RedisConnString
	if RedisMasterName == "" {
		logger.SysLog("Redis is enabled")
		opt, err := redis.ParseURL(redisConnString)
		if err != nil {
			logger.FatalLog("failed to parse Redis connection string: " + err.Error())
		}
		RDB = redis.NewClient(opt)
	} else {
		// cluster mode
		logger.SysLog("Redis cluster mode enabled")
		RDB = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:      strings.Split(redisConnString, ","),
			Password:   RedisPassword,
			MasterName: RedisMasterName,
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = RDB.Ping(ctx).Result()
	if err != nil {
		logger.FatalLog("Redis ping test failed: " + err.Error())
	}
	RedisEnabled = true
	return err
}

func ParseRedisOption() *redis.Options {
	opt, err := redis.ParseURL(RedisConnString)
	if err != nil {
		logger.FatalLog("failed to parse Redis connection string: " + err.Error())
	}
	return opt
}

func RedisSet(key string, value string, expiration time.Duration) error {
	if err := ensureRedisClient(); err != nil {
		return err
	}
	ctx := context.Background()
	return RDB.Set(ctx, key, value, expiration).Err()
}

func RedisGet(key string) (string, error) {
	if err := ensureRedisClient(); err != nil {
		return "", err
	}
	ctx := context.Background()
	return RDB.Get(ctx, key).Result()
}

func RedisDel(key string) error {
	if err := ensureRedisClient(); err != nil {
		return err
	}
	ctx := context.Background()
	return RDB.Del(ctx, key).Err()
}

func RedisDelByPattern(pattern string) error {
	if err := ensureRedisClient(); err != nil {
		return err
	}
	ctx := context.Background()
	var cursor uint64
	for {
		keys, nextCursor, err := RDB.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := RDB.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			return nil
		}
	}
}

func RedisDecrease(key string, value int64) error {
	if err := ensureRedisClient(); err != nil {
		return err
	}
	ctx := context.Background()
	return RDB.DecrBy(ctx, key, value).Err()
}
