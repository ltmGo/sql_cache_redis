package sql_cache_redis

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestMakeCacheRedis(t *testing.T) {
	runtime.GOMAXPROCS(8)
	host := ""
	pas := ""
	errs, client := NewCacheRedis(host, pas, 1, 500, 10)
	if errs != nil {
		t.Log(errs)
		return
	}
	client.NewChanelMap("test_game:1", &KeyConfig{60, 10, 100})
	var wg sync.WaitGroup
	for i := 0; i < 100 ; i ++ {
		wg.Add(1)
		go func(i int) {
			wg.Done()
			err, ok, res := client.Get("test_game:" + strconv.Itoa(i))
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
				err = client.Set("test_game:"  + strconv.Itoa(i), "123456")
				if err != nil {
					t.Log(err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	time.Sleep(time.Second*4)
}