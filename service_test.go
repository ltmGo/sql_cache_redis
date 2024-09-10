package sql_cache_redis

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestMakeCacheRedis(t *testing.T) {
	runtime.GOMAXPROCS(8)
	host := "39.98.84.55:63794"
	pas := "Yunshi@123"
	errs, client := NewCacheRedis(host, pas, 1, 500, 10)
	if errs != nil {
		t.Log(errs)
		return
	}
	client.newChanelMap("test_game:1", &KeyConfig{60, 10, 100})
	client.Del("test_game")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			wg.Done()
			err, ok, res := client.Get("test_game")
			if err != nil {
				t.Log(err)
				return
			}
			if res != nil {
				fmt.Println(i, " 获取到了值："+string(res))
				return
			}
			if ok {
				fmt.Println(i, "  得到了锁")
				time.Sleep(time.Second)
				client.DeferDo(nil, nil, "test_game", "123456", ok)
			}
		}(i)
	}
	wg.Wait()
	time.Sleep(time.Second * 4)
}
