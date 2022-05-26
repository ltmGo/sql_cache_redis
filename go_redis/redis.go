package go_redis

import (
	"github.com/go-redis/redis"
)

type RedisCfg struct {
	Addr         string //host:port address.
	Password     string
	DB           int // Database to be selected after connecting to the server.
	PoolSize     int // Maximum number of socket connections.  Default is 10 connections per every CPU as reported by runtime.NumCPU.
	MinIdleConnes int // Minimum number of idle connections which is useful when establishing new connection is slow.
}

func InitRedis(redisCfg *RedisCfg) (error, *redis.Client) {
	client := redis.NewClient(&redis.Options{
		Addr:         redisCfg.Addr,
		Password:     redisCfg.Password,
		DB:           redisCfg.DB,
		PoolSize:     redisCfg.PoolSize,
		MinIdleConns: redisCfg.MinIdleConnes,
	})
	_, err := client.Ping().Result()
	if err != nil {
		return err, nil
	} else {
		return nil, client
	}
}
