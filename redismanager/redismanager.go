package redismanager

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/neosouler7/bookstore-contango/commons"
	"github.com/neosouler7/bookstore-contango/config"
	"github.com/neosouler7/bookstore-contango/tgmanager"

	"github.com/go-redis/redis/v8"
)

var (
	ctx        = context.Background()
	rdb        *redis.Client
	once       sync.Once
	syncMap    sync.Map // to escape 'concurrent map read and map write' error
	location   *time.Location
	StampMicro = "Jan _2 15:04:05.000000"
	errMsgCnt  = 0
	errMsg     = ""
)

type premium struct {
	key   string
	value string
}

func init() {
	location = commons.SetTimeZone("Redis")
}

func client() *redis.Client {
	once.Do(func() {
		redisConfig := config.GetRedis()
		rdb = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", redisConfig.Host, redisConfig.Port),
			Password: redisConfig.Pwd,
			DB:       redisConfig.Db,
		})

		_, err := rdb.Ping(ctx).Result()
		tgmanager.HandleErr("redis", err)

		// client().FlushAll(ctx) // init flushall
	})
	return rdb
}

func PreHandlePremium(key string, value string) {
	p := newPremium(key, value)
	p.setPremium()
}

func newPremium(key string, value string) *premium {
	p := &premium{
		key:   key,
		value: value,
	}
	return p
}

func (p *premium) setPremium() {
	tradeType, exchangeA, exchangeB, market, symbol := strings.Split(p.key, ":")[0], strings.Split(p.key, ":")[1], strings.Split(p.key, ":")[2], strings.Split(p.key, ":")[3], strings.Split(p.key, ":")[4]

	// add pair info on SET
	err := client().SAdd(ctx, fmt.Sprintf("%s_target", tradeType), fmt.Sprintf("%s:%s:%s:%s", exchangeA, exchangeB, market, symbol), nil).Err()
	tgmanager.HandleErr(p.key, err)

	// set each pair's value on STRING
	err2 := client().Set(ctx, p.key, p.value, 0).Err()
	tgmanager.HandleErr(p.key, err2)

	log.Printf("REDIS SET %s -> %s\n", p.key, p.value)

}
