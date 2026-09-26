package config

import (
	"context"
	"fmt"
	"go-rich-buddy-platform/pkg/logger"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type RedisConfig struct {
	HostName string
	Password string
	Database int
}

func NewRedisConfig(hostName, password string, database int) RedisConfig {
	return RedisConfig{
		HostName: hostName,
		Password: password,
		Database: database,
	}
}

func LoadRedisConfigFromEnvironment(viperConfig *viper.Viper) (string, string, int) {
	redisHostName := viperConfig.GetString("REDIS_HOST_NAME")
	redisPassword := viperConfig.GetString("REDIS_PASSWORD")
	redisDatabaseIndex := viperConfig.GetString("REDIS_DATABASE")
	parsedRedisDatabaseIndex, err := strconv.ParseInt(redisDatabaseIndex, 10, 32)
	if err != nil {
		logger.WithError(err).Fatal("failed to parse redis database index")
	}
	return redisHostName, redisPassword, int(parsedRedisDatabaseIndex)

}

type RedisInstance struct {
	RedisClient  *redis.Client
	redisContext context.Context
}

func InitRedisInstance(redisConfig RedisConfig) (*RedisInstance, error) {
	redisContext := context.Background()
	redisDatabase := redis.NewClient(&redis.Options{
		Addr:     redisConfig.HostName,
		Password: redisConfig.Password,
		DB:       redisConfig.Database,
	})

	// Test connection
	if err := redisDatabase.Ping(redisContext).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect Redis: %w", err)
	}

	return &RedisInstance{
		RedisClient:  redisDatabase,
		redisContext: redisContext,
	}, nil
}

func (redisInstance *RedisInstance) Publish(topic string, message string) error {
	if redisInstance.RedisClient == nil {
		return fmt.Errorf("redis client is not initialized")
	}

	err := redisInstance.RedisClient.Publish(redisInstance.redisContext, topic, message).Err()
	if err != nil {
		return fmt.Errorf("failed to publish message to topic '%s': %w", topic, err)
	}

	logrus.Debug("📢 Published message to Redis topic '%s'\n", topic)
	return nil
}
