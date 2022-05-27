sql 缓存到redis中，解决并发访问db问题
缓存失效后，保证只有一个请求到db查询
每个缓存的key会保存在sync.Map中，key越多消耗的内存越大
如果key对应的channel没有初始化，将是否默认的初始化值