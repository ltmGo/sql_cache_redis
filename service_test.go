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
	errs, client := NewCacheRedis("", "", 1, 500, 10)
	if errs != nil {
		t.Log(errs)
		return
	}
	client.NewChanelMap("test_game", &KeyChannel{})
	var wg sync.WaitGroup
	for i := 0; i < 10000 ; i ++ {
		wg.Add(1)
		go func(i int) {
			wg.Done()
			err, ok, res := client.Get("test_game")
			if err != nil {
				t.Log(err)
				return
			}
			if res != "" {
				fmt.Println(i, " 获取到了值：" + res)
				return
			}
			if ok {
				fmt.Println(i, "  得到了锁")
				time.Sleep(time.Second)
				err = client.Set("test_game", "123456")
				if err != nil {
					t.Log(err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	time.Sleep(time.Second*30)
}