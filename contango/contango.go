package contango

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neosouler7/bookstore-contango/binance"
	"github.com/neosouler7/bookstore-contango/commons"
	"github.com/neosouler7/bookstore-contango/tgmanager"
	"github.com/neosouler7/bookstore-contango/websocketmanager"
)

var (
	syncMap sync.Map
)

func pong() {
	for {
		for _, exchange := range []string{"bin", "bif"} {
			websocketmanager.Pong(exchange)
		}
		time.Sleep(time.Second * 5)
	}
}

func subscribe(pairs []string) {
	time.Sleep(time.Second * 1)
	var streamSlice []string
	for _, pair := range pairs {
		var pairInfo = strings.Split(pair, ":")
		market, symbol := strings.ToLower(pairInfo[0]), strings.ToLower(pairInfo[1])

		streamSlice = append(streamSlice, fmt.Sprintf("\"%s%s@depth20\"", symbol, market))
	}
	streams := strings.Join(streamSlice, ",")
	msg := fmt.Sprintf("{\"method\": \"SUBSCRIBE\",\"params\": [%s],\"id\": %d}", streams, time.Now().UnixNano()/100000)

	for _, exchange := range []string{"bin", "bif"} {
		websocketmanager.SendMsg(exchange, msg)
		fmt.Printf(websocketmanager.SubscribeMsg, exchange)
	}

}

func receiveBin(exchange string) {
	for {
		_, msgBytes, err := websocketmanager.Conn(exchange).ReadMessage()
		tgmanager.HandleErr(exchange, err)

		if strings.Contains(string(msgBytes), "result") {
			fmt.Printf(websocketmanager.FilteredMsg, exchange, string(msgBytes))
		} else {
			var rJson interface{}
			commons.Bytes2Json(msgBytes, &rJson)

			data := rJson.(map[string]interface{})["data"]
			askR, bidR := data.(map[string]interface{})["asks"], data.(map[string]interface{})["bids"]
			askPrice, bidPrice := askR.([]interface{})[1].([]interface{})[0], bidR.([]interface{})[1].([]interface{})[0] // 2번째 호가의 가격
			ts, pair := strconv.FormatInt(time.Now().UnixNano()/1000000, 10), strings.Split(rJson.(map[string]interface{})["stream"].(string), "@")[0]
			market, symbol := "usdt", pair[:len(pair)-4]

			// fmt.Printf("%s|%s|%s|%s|%s|%s\n", exchange, market, symbol, askPrice, bidPrice, ts)

			key := fmt.Sprintf("%s:%s:%s", exchange, market, symbol)
			value := fmt.Sprintf("%s:%s%s", askPrice, bidPrice, ts)
			syncMap.Store(key, value)
		}

	}
}

func receiveBif(exchange string) {
	for {
		_, msgBytes, err := websocketmanager.Conn(exchange).ReadMessage()
		tgmanager.HandleErr(exchange, err)

		if strings.Contains(string(msgBytes), "result") {
			fmt.Printf(websocketmanager.FilteredMsg, exchange, string(msgBytes))
		} else {
			var rJson interface{}
			commons.Bytes2Json(msgBytes, &rJson)

			data := rJson.(map[string]interface{})["data"]
			askR, bidR := data.(map[string]interface{})["a"], data.(map[string]interface{})["b"]
			askPrice, bidPrice := askR.([]interface{})[1].([]interface{})[0], bidR.([]interface{})[1].([]interface{})[0] // 2번째 호가의 가격
			ts, pair := strconv.FormatInt(int64(data.(map[string]interface{})["T"].(float64)), 10), data.(map[string]interface{})["s"]
			market, symbol := "usdt", strings.ToLower(pair.(string)[:len(pair.(string))-4])

			// fmt.Printf("%s|%s|%s|%s|%s|%s\n", exchange, market, symbol, askPrice, bidPrice, ts)

			key := fmt.Sprintf("%s:%s:%s", exchange, market, symbol)
			value := fmt.Sprintf("%s:%s%s", askPrice, bidPrice, ts)
			syncMap.Store(key, value)
		}

	}
}

func setPremium(pairs []string) {
	for {
		for _, pair := range pairs {
			var pairInfo = strings.Split(pair, ":")
			market, symbol := strings.ToLower(pairInfo[0]), strings.ToLower(pairInfo[1])

			spot, ok := waitForSyncMap(fmt.Sprintf("bin:%s:%s", market, symbol))
			if !ok {
				fmt.Printf("Spot data not ready for %s:%s, retrying...\n", market, symbol)
				time.Sleep(time.Second * 1)
				continue
			}

			future, ok := waitForSyncMap(fmt.Sprintf("bif:%s:%s", market, symbol))
			if !ok {
				fmt.Printf("Future data not ready for %s:%s, retrying...\n", market, symbol)
				time.Sleep(time.Second * 1)
				continue
			}

			// Premium 계산
			spotPrice, futurePrice := strings.Split(spot.(string), ":")[0], strings.Split(future.(string), ":")[1]
			spotPriceFloat, _ := strconv.ParseFloat(spotPrice, 64)
			futurePriceFloat, _ := strconv.ParseFloat(futurePrice, 64)
			premium := commons.RoundToDecimal((futurePriceFloat-spotPriceFloat)/spotPriceFloat, 5) * 100
			// signal := ""
			// if premium > 0 {
			// 	signal = "plus"
			// } else {
			// 	signal = "minus"
			// }

			key := fmt.Sprintf("%s:%s", market, symbol)
			value := fmt.Sprintf("%f", premium)
			msg := fmt.Sprintf("%s -> %.6s (spot: %.8s, future: %.8s)\n", key, value, spotPrice, futurePrice)

			fmt.Printf("%s", msg)
			// 특정 조건에 따라 메세지 전송
			if premium > 0.3 {
				tgmanager.SendMsg(msg)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func waitForSyncMap(key string) (interface{}, bool) {
	for {
		value, ok := syncMap.Load(key)
		if ok {
			return value, true
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func intersect(slice1, slice2 []string) []string {
	// 첫 번째 슬라이스 요소를 map에 저장
	lookup := make(map[string]struct{})
	for _, item := range slice1 {
		lookup[item] = struct{}{}
	}

	// 공통된 요소를 저장할 결과 슬라이스
	var result []string
	for _, item := range slice2 {
		if _, exists := lookup[item]; exists {
			result = append(result, item)
			delete(lookup, item) // 중복 방지를 위해 제거
		}
	}

	return result
}

func getBinBifMutualPairs() []string {
	binPairs, bifPairs := binance.GetBinExchangeInfo(), binance.GetBifExchangeInfo()
	mutualPairs := intersect(binPairs, bifPairs)
	fmt.Printf("bin: %d, bif: %d, mut: %d\n", len(binPairs), len(bifPairs), len(mutualPairs))
	return mutualPairs
}

func Run() {
	// TODO. bybit -> byb, byf
	// TODO. key update -> bin:bif:usdt:xrp:0.05:ts, byb:bif:usdt:btc:0.05:ts
	// config {future: ["bif", "byf"], spot: ["bin", "byb"] }

	binBifMutualPairs := getBinBifMutualPairs()[:200]
	// pairs := []string{"usdt:xtz", "usdt:unfi", "usdt:celr"}
	var wg sync.WaitGroup

	// pong
	wg.Add(1)
	go pong()

	// subscribe websocket stream
	wg.Add(1)
	go func() {
		subscribe(binBifMutualPairs)
		wg.Done()
	}()

	// receive websocket msg
	wg.Add(1)
	go receiveBin("bin")

	wg.Add(1)
	go receiveBif("bif")

	// calculate premium
	wg.Add(1)
	go setPremium(binBifMutualPairs)
	// go func() {
	// 	setPremium(pairs)
	// 	wg.Done()
	// }()

	wg.Wait()
}
