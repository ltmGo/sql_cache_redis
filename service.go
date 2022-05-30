package sql_cache_redis

import (
	"context"
	"errors"
	"github.com/go-redis/redis"
	"github.com/ltmGo/sql_cache_redis/go_redis"
	"strings"
	"sync"
	"time"
)


const (
	sleepMillisecondsDefault = 60 //默认休眠的时间，毫秒
	cacheSecondsDefault = 120 //秒
	responseSecondsDefault = 10 //超时10秒
)

type KeyConfig struct {
	CacheSeconds uint //缓存过期时间
	ResponseSeconds uint //响应超时时间
	SleepMilliseconds uint //默认休眠时间，根据查询时间自己计算
}

//KeyChannel redis的缓存key
type keyChannel struct {
	ch chan struct{}
	cacheSeconds time.Duration //缓存过期时间
	responseSeconds time.Duration //响应超时时间
	sleepMilliseconds time.Duration //默认休眠时间，根据查询时间自己计算
}

type CacheRedis struct {
	ChannelMap sync.Map
	Client *redis.Client
}

//NewCacheRedis 构造函数
func NewCacheRedis(redisHost, redisPwd string, redisDb, redisPool, MinIdleConnes int) (error, *CacheRedis) {
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
	return nil, &CacheRedis{
		ChannelMap: sync.Map{},
		Client:  client,
	}
}

//NewChanelMap 创建channel通道
func (c *CacheRedis) NewChanelMap(key string, configChannel *KeyConfig) *keyChannel{
	cacheKey, ok := c.ChannelMap.Load(key)
	//如果不存在就创建
	if !ok {
		//创建一个有缓冲的通道
		return c.newKeyChannel(key, configChannel)
	}else {
		return cacheKey.(*keyChannel)
	}
}

//checkKeyChannel 检验key参数
func (k *KeyConfig) checkKeyChannel() {
	if k.CacheSeconds == 0 {
		k.CacheSeconds = cacheSecondsDefault
	}
	if k.ResponseSeconds == 0 {
		k.ResponseSeconds = responseSecondsDefault
	}
	if k.SleepMilliseconds == 0 {
		k.SleepMilliseconds = sleepMillisecondsDefault
	}
}

//newKeyChannel 使用sync.Map创建，避免并发情况下重复创建cache的channel单例，利用sync.Map的存在就不覆盖
//如果有条件，每个缓存可以先初始化通道
func (c *CacheRedis) newKeyChannel(key string, configChannel *KeyConfig) *keyChannel {
	chBool := make(chan struct{}, 1)
	chBool <- struct {}{}
	configChannel.checkKeyChannel()
	cacheKey := &keyChannel{
		ch: chBool,
		cacheSeconds: time.Duration(configChannel.CacheSeconds) * time.Second,
		responseSeconds: time.Duration(configChannel.ResponseSeconds) * time.Second,
		sleepMilliseconds: time.Duration(configChannel.SleepMilliseconds) * time.Millisecond,
	}
	//time.Sleep(time.Millisecond*10)
	//若key已存在，则返回true和key对应的value，不会修改原来的value
	exists, _ := c.ChannelMap.LoadOrStore(key, cacheKey)
	//==========================验证sync.Map是否重复创建覆盖以前的实例==========================
	//if ok {
	//	fmt.Println(key, " 已被创建, 后获取的时间戳", exists.(*keyChannel).time)
	//}else {
	//	fmt.Println(key, " 第一次创建的时间戳", cacheKey.time)
	//}
	return exists.(*keyChannel)
}


//GetChanel 获取key的通道
func (c *CacheRedis) getChanel(key string) *keyChannel {
	return c.NewChanelMap(key, &KeyConfig{})
}


//GetValues 从redis中获取value
func (c *CacheRedis) getValues(key string) ([]byte, error) {
	res, err := c.Client.Get(key).Bytes()
	if err != nil {
		if strings.Contains(err.Error(), "redis: nil") {
			return nil, nil
		}
	}
	return res, err
}

//SetChannelValues 修改channel状态
func (c *CacheRedis) setChannelValues(key string) {
	c.getChanel(key).ch <- struct{}{}
}


//Set redis设置缓存的值，过期时间也可以重新设置
func (c *CacheRedis) Set(key string, values []byte, expire ...uint) error {
	var ex time.Duration
	if len(expire) == 1 {
		ex = time.Second * time.Duration(expire[0])
	}else {
		ex = c.getChanel(key).cacheSeconds
	}
	err := c.Client.Set(key, values, ex).Err()
	if err == nil {
		//多增加了10毫秒，是为了避免阻塞的协程，重复访问db并写缓存
		time.Sleep(c.getChanel(key).sleepMilliseconds + 10)
	}
	c.setChannelValues(key)
	return err
}

//Get 根据sql的查询时间，指定每次休眠的时间毫秒
func (c *CacheRedis) Get(key string, sleep ...uint) (err error, ok bool, res []byte){
	ch := c.getChanel(key)
	ctx, cancel := context.WithTimeout(context.Background(), ch.responseSeconds)
	defer cancel()
	var ex time.Duration
	if len(sleep) == 1 {
		ex = time.Millisecond * time.Duration(sleep[0])
	}else {
		ex = ch.sleepMilliseconds
	}
	for {
		res, err = c.getValues(key)
		if err != nil || res != nil {
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
			time.Sleep(ex)
		}
	}
}
