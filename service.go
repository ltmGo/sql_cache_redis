package sql_cache_redis

import (
	"context"
	"errors"
	"github.com/go-redis/redis"
	"github.com/ltmGo/sql_cache_redis/go_redis"
	"strings"
	"time"
)


const (
	sleepMillisecondsDefault = 100 //默认休眠的时间，毫秒
	cacheSecondsDefault = 120 //秒
	responseSecondsDefault = 30 //超时30秒
)


//KeyChannel redis的缓存key
type KeyChannel struct {
	ch chan struct{}
	cacheSeconds time.Duration //缓存过期时间
	responseSeconds time.Duration //响应超时时间
	sleepMilliseconds time.Duration //默认休眠时间，根据查询时间自己计算
}

type cacheRedis struct {
	ChannelMap map[string]*KeyChannel
	Client *redis.Client
}

//NewCacheRedis 构造函数
func NewCacheRedis(redisHost, redisPwd string, redisDb, redisPool, MinIdleConnes int) (error, *cacheRedis) {
	redisCfg := &go_redis.RedisCfg{
		Addr: redisHost,
		Password: redisPwd,
		DB: redisDb,
		PoolSize: redisPool,
		MinIdleConnes: MinIdleConnes,
	}
	//初始化redis
	err, client := go_redis.InitRedis(redisCfg)
	if err != nil {
		return err, nil
	}
	return nil, &cacheRedis{
		ChannelMap: make(map[string]*KeyChannel),
		Client:  client,
	}
}

//checkKeyChannel 检验key参数
func (k *KeyChannel) checkKeyChannel() {
	if k.cacheSeconds == 0 {
		k.cacheSeconds = cacheSecondsDefault
	}
	if k.responseSeconds == 0 {
		k.responseSeconds = responseSecondsDefault
	}
	if k.sleepMilliseconds == 0 {
		k.sleepMilliseconds = sleepMillisecondsDefault
	}
}


//NewChanelMap 创建channel通道
func (c *cacheRedis) NewChanelMap(key string, configChannel *KeyChannel) {
	_, ok := c.ChannelMap[key]
	//如果不存在就创建
	if !ok {
		//创建一个有缓冲的通道
		chBool := make(chan struct{}, 1)
		chBool <- struct {}{}
		configChannel.checkKeyChannel()
		cacheKey := &KeyChannel{
			ch: chBool,
			cacheSeconds: configChannel.cacheSeconds*time.Second,
			responseSeconds: configChannel.responseSeconds*time.Second,
			sleepMilliseconds: configChannel.sleepMilliseconds*time.Millisecond,
		}
		c.ChannelMap[key] = cacheKey
	}
	return
}


//checkChannelExists 检查key的通道是否存在
func (c *cacheRedis) checkChannelExists(key string) error {
	_, ok := c.ChannelMap[key]
	//如果不存在就创建
	if !ok {
		return errors.New(key + " key未初始化")
	}
	return nil
}

//GetChanel 获取key的通道
func (c *cacheRedis) getChanel(key string) *KeyChannel {
	cacheKey, _ := c.ChannelMap[key]
	return cacheKey
}


//GetValues 从redis中获取value
func (c *cacheRedis) getValues(key string) (string, error) {
	res, err := c.Client.Get(key).Result()
	if err != nil {
		if strings.Contains(err.Error(), "redis: nil") {
			return "", nil
		}
	}
	return res, err
}

//SetChannelValues 修改channel状态
func (c *cacheRedis) setChannelValues(key string) {
	c.getChanel(key).ch <- struct{}{}
}


//Set redis设置缓存的值
func (c *cacheRedis) Set(key, values string) error {
	err := c.checkChannelExists(key)
	if err != nil {
		return err
	}
	err = c.Client.Set(key, values, c.getChanel(key).cacheSeconds).Err()
	if err == nil {
		//多增加了10毫秒，是为了避免阻塞的协程，重复访问db并写缓存
		time.Sleep(c.getChanel(key).sleepMilliseconds + 10)
	}
	c.setChannelValues(key)
	return err
}


func (c *cacheRedis) Get(key string) (err error, ok bool, res string){
	err = c.checkChannelExists(key)
	if err != nil {
		return
	}
	ch := c.getChanel(key)
	ctx, cancel := context.WithTimeout(context.Background(), ch.responseSeconds)
	defer cancel()
	for {
		res, err = c.getValues(key)
		if err != nil || res != "" {
			return
		}
		select {
		case  <- c.getChanel(key).ch:
			//执行sql查询语句，并更新缓存
			ok = true
			return
		case <- ctx.Done():
			//超时取消
			err = errors.New("查询缓存超时")
			return
		default:
			//休眠毫秒
			time.Sleep(ch.sleepMilliseconds)
		}
	}
}
