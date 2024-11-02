package contango

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neosouler7/bookstore-contango/binance"
	"github.com/neosouler7/bookstore-contango/bybit"
	"github.com/neosouler7/bookstore-contango/commons"
	"github.com/neosouler7/bookstore-contango/tgmanager"
	"github.com/neosouler7/bookstore-contango/websocketmanager"
)

var (
	syncMap      sync.Map
	bybOrderbook sync.Map
	byfOrderbook sync.Map
)

type OrderbookEntry struct {
	Price  float64
	Amount float64
}

type Orderbook struct {
	Bids []OrderbookEntry // 내림차순
	Asks []OrderbookEntry // 오름차순
}

func pong() {
	for {
		for _, exchange := range []string{"bin", "bif"} {
			websocketmanager.Pong(exchange)
		}
		for _, exchange := range []string{"byb", "byf"} {
			websocketmanager.SendMsg(exchange, "{\"op\": \"ping\"")
		}
		log.Println("ping")
		time.Sleep(time.Second * 5)
	}
}

func subscribe(pairs []string) {
	time.Sleep(time.Second * 1)
	for _, exchange := range []string{"bin", "bif"} {
		var streamSlice []string
		for _, pair := range pairs {
			var pairInfo = strings.Split(pair, ":")
			market, symbol := strings.ToLower(pairInfo[0]), strings.ToLower(pairInfo[1])
			streamSlice = append(streamSlice, fmt.Sprintf("\"%s%s@depth20\"", symbol, market))
		}
		streams := strings.Join(streamSlice, ",")

		msg := fmt.Sprintf("{\"method\": \"SUBSCRIBE\",\"params\": [%s],\"id\": %d}", streams, time.Now().UnixNano()/100000)
		websocketmanager.SendMsg(exchange, msg)
		log.Printf(websocketmanager.SubscribeMsg, exchange)
	}

	for _, exchange := range []string{"byb", "byf"} {
		var streamSlice []string
		for _, pair := range pairs {
			var pairInfo = strings.Split(pair, ":")
			market, symbol := strings.ToUpper(pairInfo[0]), strings.ToUpper(pairInfo[1])
			streamSlice = append(streamSlice, fmt.Sprintf("\"orderbook.50.%s%s\"", symbol, market))
		}

		for i := 0; i < len(streamSlice); i += 5 {
			end := i + 5
			if end > len(streamSlice) {
				end = len(streamSlice)
			}
			streams := strings.Join(streamSlice[i:end], ",")

			msg := fmt.Sprintf("{\"op\": \"subscribe\",\"args\": [%s]}", streams)
			//fmt.Println(msg)

			websocketmanager.SendMsg(exchange, msg)
			fmt.Printf(websocketmanager.SubscribeMsg, exchange)
		}
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

			//fmt.Printf("%s|%s|%s|%s|%s|%s\n", exchange, market, symbol, askPrice, bidPrice, ts)

			key := fmt.Sprintf("%s:%s:%s", exchange, market, symbol)
			value := fmt.Sprintf("%s:%s:%s", askPrice, bidPrice, ts)
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

			//fmt.Printf("%s|%s|%s|%s|%s|%s\n", exchange, market, symbol, askPrice, bidPrice, ts)

			key := fmt.Sprintf("%s:%s:%s", exchange, market, symbol)
			value := fmt.Sprintf("%s:%s:%s", askPrice, bidPrice, ts)
			syncMap.Store(key, value)
		}

	}
}

func UpdateOrderbook(orderbookMap *sync.Map, pair string, isSnapshot bool, bids, asks [][]string) {
	var currentOrderbook Orderbook

	// Orderbook이 이미 존재하는지 확인
	if val, ok := orderbookMap.Load(pair); ok {
		currentOrderbook, _ = val.(Orderbook)
	}

	// Bids 처리
	for _, bid := range bids {
		price, _ := strconv.ParseFloat(bid[0], 64)
		amount, _ := strconv.ParseFloat(bid[1], 64)

		// Amount가 0이면 제거
		if amount == 0 {
			currentOrderbook.Bids = removeOrder(currentOrderbook.Bids, price)
		} else {
			currentOrderbook.Bids = updateOrder(currentOrderbook.Bids, price, amount)
		}
	}

	// Asks 처리
	for _, ask := range asks {
		price, _ := strconv.ParseFloat(ask[0], 64)
		amount, _ := strconv.ParseFloat(ask[1], 64)

		// Amount가 0이면 제거
		if amount == 0 {
			currentOrderbook.Asks = removeOrder(currentOrderbook.Asks, price)
		} else {
			currentOrderbook.Asks = updateOrder(currentOrderbook.Asks, price, amount)
		}
	}

	// 로컬 orderbookMap에 업데이트된 Orderbook 저장
	orderbookMap.Store(pair, currentOrderbook)
}

// Order를 업데이트하거나 추가하는 함수
func updateOrder(orders []OrderbookEntry, price, amount float64) []OrderbookEntry {
	for i, order := range orders {
		if order.Price == price {
			// 가격이 존재하면 업데이트
			orders[i].Amount = amount
			return orders
		}
	}
	// 가격이 존재하지 않으면 추가
	orders = append(orders, OrderbookEntry{Price: price, Amount: amount})
	return orders
}

// 특정 가격의 Order를 제거하는 함수
func removeOrder(orders []OrderbookEntry, price float64) []OrderbookEntry {
	for i, order := range orders {
		if order.Price == price {
			return append(orders[:i], orders[i+1:]...)
		}
	}
	return orders
}

// 현재 askPrice 및 bidPrice 가져오기
func getBestPrices(orderbookMap *sync.Map, pair string) (float64, float64) {
	val, ok := orderbookMap.Load(pair)
	if !ok {
		return 0, 0 // Orderbook이 없는 경우 0 반환
	}
	orderbook := val.(Orderbook)

	// 최소값(최고 Bid), 최대값(최저 Ask) 찾기
	bestBid := 0.0
	if len(orderbook.Bids) > 0 {
		for _, bid := range orderbook.Bids {
			if bid.Price > bestBid {
				bestBid = bid.Price
			}
		}
	}

	bestAsk := 0.0
	if len(orderbook.Asks) > 0 {
		bestAsk = orderbook.Asks[0].Price // 첫 번째 요소가 최소값
		for _, ask := range orderbook.Asks {
			if ask.Price < bestAsk {
				bestAsk = ask.Price
			}
		}
	}

	return bestBid, bestAsk
}

func parseOrders(rawOrders []interface{}) [][]string {
	var orders [][]string
	for _, rawOrder := range rawOrders {
		order := rawOrder.([]interface{})
		price := order[0].(string)
		amount := order[1].(string)
		orders = append(orders, []string{price, amount})
	}
	return orders
}

func receiveByb(exchange string) {
	for {
		_, msgBytes, err := websocketmanager.Conn(exchange).ReadMessage()
		tgmanager.HandleErr(exchange, err)

		var rJson map[string]interface{}
		commons.Bytes2Json(msgBytes, &rJson)

		if _, ok := rJson["topic"].(string); ok {
			if data, ok := rJson["data"].(map[string]interface{}); ok {
				pair, ts := data["s"].(string), rJson["ts"].(float64)
				market, symbol := "usdt", strings.ToLower(pair[:len(pair)-len("USDT")])

				// Orderbook Update
				bids := parseOrders(data["b"].([]interface{}))
				asks := parseOrders(data["a"].([]interface{}))
				isSnapshot := rJson["type"].(string) == "snapshot"
				UpdateOrderbook(&bybOrderbook, pair, isSnapshot, bids, asks)

				// 현재 ask/bid 가격 출력
				bidPrice, askPrice := getBestPrices(&bybOrderbook, pair)
				bidPriceStr, askPriceStr := strconv.FormatFloat(bidPrice, 'f', -1, 64), strconv.FormatFloat(askPrice, 'f', -1, 64)
				tsStr := strconv.FormatFloat(ts, 'f', -1, 64)
				//fmt.Printf("%s|%s|%s|%s|%s|%s\n", exchange, market, symbol, askPriceStr, bidPriceStr, tsStr)

				key := fmt.Sprintf("%s:%s:%s", exchange, market, symbol)
				value := fmt.Sprintf("%s:%s:%s", askPriceStr, bidPriceStr, tsStr)
				syncMap.Store(key, value)
			}
		}
	}
}

func receiveByf(exchange string) {
	for {
		_, msgBytes, err := websocketmanager.Conn(exchange).ReadMessage()
		tgmanager.HandleErr(exchange, err)

		var rJson map[string]interface{}
		commons.Bytes2Json(msgBytes, &rJson)

		if _, ok := rJson["topic"].(string); ok {
			if data, ok := rJson["data"].(map[string]interface{}); ok {
				pair, ts := data["s"].(string), rJson["ts"].(float64)
				market, symbol := "usdt", strings.ToLower(pair[:len(pair)-len("USDT")])

				// Orderbook Update
				bids := parseOrders(data["b"].([]interface{}))
				asks := parseOrders(data["a"].([]interface{}))
				isSnapshot := rJson["type"].(string) == "snapshot"
				UpdateOrderbook(&byfOrderbook, pair, isSnapshot, bids, asks)

				// 현재 ask/bid 가격 출력
				bidPrice, askPrice := getBestPrices(&byfOrderbook, pair)
				bidPriceStr, askPriceStr := strconv.FormatFloat(bidPrice, 'f', -1, 64), strconv.FormatFloat(askPrice, 'f', -1, 64)
				tsStr := strconv.FormatFloat(ts, 'f', -1, 64)
				//fmt.Printf("%s|%s|%s|%s|%s|%s\n", exchange, market, symbol, askPriceStr, bidPriceStr, tsStr)

				key := fmt.Sprintf("%s:%s:%s", exchange, market, symbol)
				value := fmt.Sprintf("%s:%s:%s", askPriceStr, bidPriceStr, tsStr)
				syncMap.Store(key, value)
			}
		}
	}
}

func setPremium(pairs []string) {
	for {
		for _, pair := range pairs {
			var pairInfo = strings.Split(pair, ":")
			market, symbol := strings.ToLower(pairInfo[0]), strings.ToLower(pairInfo[1])

			// for _, se := range []string{"bin", "byb"} {
			// 	for _, fe := range []string{"bif", "byf"} {
			for _, se := range []string{"bin", "byb"} {
				for _, fe := range []string{"bif", "byf"} {
					// TODO. 값을 못 가져옴
					spot, ok := waitForSyncMap(fmt.Sprintf("%s:%s:%s", se, market, symbol))
					if !ok {
						fmt.Printf("Spot data not ready for %s:%s:%s, retrying...\n", se, market, symbol)
						time.Sleep(time.Second * 1)
						continue
					}

					future, ok := waitForSyncMap(fmt.Sprintf("%s:%s:%s", fe, market, symbol))
					if !ok {
						fmt.Printf("Future data not ready for %s:%s:%s, retrying...\n", fe, market, symbol)
						time.Sleep(time.Second * 1)
						continue
					}

					// Premium 계산
					spotPrice, futurePrice, _ := strings.Split(spot.(string), ":")[0], strings.Split(future.(string), ":")[1], strings.Split(future.(string), ":")[2]
					spotPriceFloat, _ := strconv.ParseFloat(spotPrice, 64)
					futurePriceFloat, _ := strconv.ParseFloat(futurePrice, 64)
					premium := commons.RoundToDecimal((futurePriceFloat-spotPriceFloat)/spotPriceFloat, 5) * 100

					key := fmt.Sprintf("%s:%s:%s:%s", se, fe, market, symbol)
					value := fmt.Sprintf("%f", premium)
					msg := fmt.Sprintf("%s -> %.6s (spot: %.8s, future: %.8s)\n", key, value, spotPrice, futurePrice)

					log.Printf("%s", msg)

					if premium > 0.3 {
						tgmanager.SendMsg(msg)
					}
					// TODO. redis set
					time.Sleep(10 * time.Millisecond)
				}
			}
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

// 편의상 모든 거래소에 공통으로 등록된 symbol을 대상으로함. 그럼에도 충분하다!
func getMutualPairs() []string {
	binPairs, bifPairs := binance.GetBinExchangeInfo(), binance.GetBifExchangeInfo()
	bybPairs, byfPairs := bybit.GetBybExchangeInfo(), bybit.GetByfExchangeInfo()
	mutualPairs := intersect(intersect(binPairs, bifPairs), intersect(bybPairs, byfPairs))
	log.Printf("bin: %d, bif: %d, byb: %d, byf: %d -> mut: %d\n", len(binPairs), len(bifPairs), len(bybPairs), len(byfPairs), len(mutualPairs))
	return mutualPairs
}

func Run() {
	pairs := getMutualPairs()[:100]
	// pairs := []string{"usdt:xtz", "usdt:unfi", "usdt:celr"}
	var wg sync.WaitGroup

	// pong
	wg.Add(1)
	go pong()

	// subscribe websocket stream
	wg.Add(1)
	go func() {
		subscribe(pairs)
		wg.Done()
	}()

	// receive websocket msg
	wg.Add(1)
	go receiveBin("bin")

	wg.Add(1)
	go receiveBif("bif")

	wg.Add(1)
	go receiveByb("byb")

	wg.Add(1)
	go receiveByf("byf")

	// calculate premium
	wg.Add(1)
	go setPremium(pairs)

	wg.Wait()
}
