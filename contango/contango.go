package contango

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/neosouler7/bookstore-contango/binance"
	"github.com/neosouler7/bookstore-contango/bybit"
	"github.com/neosouler7/bookstore-contango/commons"
	"github.com/neosouler7/bookstore-contango/redismanager"
	"github.com/neosouler7/bookstore-contango/tgmanager"
	"github.com/neosouler7/bookstore-contango/websocketmanager"
)

var (
	subscribeDone = make(chan struct{}) // subscribe 완료 신호 채널
	bybOrderbook  sync.Map
	byfOrderbook  sync.Map
	syncMap       sync.Map
)

type OrderbookEntry struct {
	Price  float64
	Amount float64
}

type Orderbook struct {
	Bids []OrderbookEntry // 내림차순
	Asks []OrderbookEntry // 오름차순
}

// byb, byf의 websocket 리턴값을 적정 반영하기 위한 함수
func updateOrderbook(orderbookMap *sync.Map, pair string, bids, asks [][]string) {
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

// Order 리스트를 반환하는 함수
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

func pong() {
	for {
		for _, exchange := range []string{"bin", "bif"} {
			websocketmanager.Pong(exchange)
		}
		for _, exchange := range []string{"byb", "byf"} {
			websocketmanager.SendMsg(exchange, "{\"op\": \"ping\"")

			// 추후 문제 생기면 아래 코드 참고
			// pingMessage := map[string]interface{}{
			// 	"req_id": "100001", // 사용자 정의 ID
			// 	"op":     "ping",
			// }
			// messageBytes, err := json.Marshal(pingMessage)
			// if err != nil {
			// 	log.Printf("Failed to encode ping message: %v", err)
			// 	continue
			// }

			// // Ping 메시지 전송
			// err = conn.WriteMessage(websocket.TextMessage, messageBytes)
			// if err != nil {
			// 	log.Printf("Failed to send Ping message: %v", err)
			// 	return
			// }
		}
		// log.Println("ping")
		time.Sleep(time.Second * 5)
	}
}

func subscribe(pairMap map[string][]string) {
	time.Sleep(time.Second * 1)
	for _, exchange := range []string{"bin", "bif"} {
		var streamSlice []string
		for _, pair := range pairMap[exchange] {
			var pairInfo = strings.Split(pair, ":")
			market, symbol := strings.ToLower(pairInfo[0]), strings.ToLower(pairInfo[1])
			streamSlice = append(streamSlice, fmt.Sprintf("\"%s%s@depth20\"", symbol, market))
		}

		for i := 0; i < len(streamSlice); i += 200 {
			end := i + 200
			if end > len(streamSlice) {
				end = len(streamSlice)
			}
			streams := strings.Join(streamSlice[i:end], ",")

			msg := fmt.Sprintf("{\"method\": \"SUBSCRIBE\",\"params\": [%s],\"id\": %d}", streams, time.Now().UnixNano()/100000)
			websocketmanager.SendMsg(exchange, msg)
			log.Printf(websocketmanager.SubscribeMsg, exchange)

			time.Sleep(10 * time.Millisecond)
		}
	}

	for _, exchange := range []string{"byb", "byf"} {
		var streamSlice []string
		for _, pair := range pairMap[exchange] {
			var pairInfo = strings.Split(pair, ":")
			market, symbol := strings.ToUpper(pairInfo[0]), strings.ToUpper(pairInfo[1])
			streamSlice = append(streamSlice, fmt.Sprintf("\"orderbook.50.%s%s\"", symbol, market))
		}

		for i := 0; i < len(streamSlice); i += 10 {
			end := i + 10
			if end > len(streamSlice) {
				end = len(streamSlice)
			}
			streams := strings.Join(streamSlice[i:end], ",")

			msg := fmt.Sprintf("{\"op\": \"subscribe\",\"args\": [%s]}", streams)
			websocketmanager.SendMsg(exchange, msg)
			fmt.Printf(websocketmanager.SubscribeMsg, exchange)

			time.Sleep(10 * time.Millisecond)
		}
	}

	close(subscribeDone) // 모든 websocket 구독 요청 완료
}

// func receiveBin(exchange string) {
// 	for {
// 		_, msgBytes, err := websocketmanager.Conn(exchange).ReadMessage()
// 		tgmanager.HandleErr(exchange, err)

// 		if strings.Contains(string(msgBytes), "result") {
// 			fmt.Printf(websocketmanager.FilteredMsg, exchange, string(msgBytes))
// 		} else {
// 			var rJson interface{}
// 			commons.Bytes2Json(msgBytes, &rJson)

// 			data := rJson.(map[string]interface{})["data"]
// 			askR, bidR := data.(map[string]interface{})["asks"], data.(map[string]interface{})["bids"]
// 			askPrice, bidPrice := askR.([]interface{})[1].([]interface{})[0], bidR.([]interface{})[1].([]interface{})[0] // 2번째 호가의 가격
// 			ts, pair := strconv.FormatInt(time.Now().UnixNano()/1000000, 10), strings.Split(rJson.(map[string]interface{})["stream"].(string), "@")[0]
// 			market, symbol := "usdt", pair[:len(pair)-4]

// 			//fmt.Printf("%s|%s|%s|%s|%s|%s\n", exchange, market, symbol, askPrice, bidPrice, ts)

// 			key := fmt.Sprintf("%s:%s:%s", exchange, market, symbol)
// 			value := fmt.Sprintf("%s:%s:%s", askPrice, bidPrice, ts)
// 			syncMap.Store(key, value)
// 		}
// 	}
// }

func receiveBif(exchange string) {
	for {
		msgType, msgBytes, err := websocketmanager.Conn(exchange).ReadMessage()
		tgmanager.HandleErr(exchange, err)

		switch msgType {
		case websocket.PingMessage: // 실질적으로는 거치지 않음...
			err = websocketmanager.Conn(exchange).WriteMessage(websocket.PongMessage, msgBytes)
			if err != nil {
				tgmanager.HandleErr(exchange, fmt.Errorf("failed to send Pong: %v", err))
			}
			tgmanager.SendMsg(fmt.Sprintf("%s received PING", exchange))
			continue

		case websocket.TextMessage, websocket.BinaryMessage:
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
				value := fmt.Sprintf("%s:%s:%s", askPrice, bidPrice, ts)
				syncMap.Store(key, value)
			}
		}
	}
}

// func receiveByb(exchange string) {
// 	for {
// 		_, msgBytes, err := websocketmanager.Conn(exchange).ReadMessage()
// 		tgmanager.HandleErr(exchange, err)

// 		var rJson map[string]interface{}
// 		commons.Bytes2Json(msgBytes, &rJson)

// 		if _, ok := rJson["topic"].(string); ok {
// 			if data, ok := rJson["data"].(map[string]interface{}); ok {
// 				pair, ts := data["s"].(string), rJson["ts"].(float64)
// 				market, symbol := "usdt", strings.ToLower(pair[:len(pair)-len("USDT")])

// 				// Orderbook Update
// 				bids := parseOrders(data["b"].([]interface{}))
// 				asks := parseOrders(data["a"].([]interface{}))
// 				// isSnapshot := rJson["type"].(string) == "snapshot"
// 				updateOrderbook(&bybOrderbook, pair, bids, asks)

// 				// 현재 ask/bid 가격 출력
// 				bidPrice, askPrice := getBestPrices(&bybOrderbook, pair)
// 				bidPriceStr, askPriceStr := strconv.FormatFloat(bidPrice, 'f', -1, 64), strconv.FormatFloat(askPrice, 'f', -1, 64)
// 				tsStr := strconv.FormatFloat(ts, 'f', -1, 64)

// 				//fmt.Printf("%s|%s|%s|%s|%s|%s\n", exchange, market, symbol, askPriceStr, bidPriceStr, tsStr)

// 				key := fmt.Sprintf("%s:%s:%s", exchange, market, symbol)
// 				value := fmt.Sprintf("%s:%s:%s", askPriceStr, bidPriceStr, tsStr)
// 				syncMap.Store(key, value)
// 			}
// 		}
// 	}
// }

func receiveByf(exchange string) {
	for {
		msgType, msgBytes, err := websocketmanager.Conn(exchange).ReadMessage()
		tgmanager.HandleErr(exchange, err)

		switch msgType {
		case websocket.PingMessage: // 실질적으로는 거치지 않음...
			err = websocketmanager.Conn(exchange).WriteMessage(websocket.PongMessage, msgBytes)
			if err != nil {
				tgmanager.HandleErr(exchange, fmt.Errorf("failed to send Pong: %v", err))
			}
			tgmanager.SendMsg(fmt.Sprintf("%s received PING", exchange))
			continue

		case websocket.TextMessage, websocket.BinaryMessage:
			var rJson map[string]interface{}
			commons.Bytes2Json(msgBytes, &rJson)

			if _, ok := rJson["topic"].(string); ok {
				if data, ok := rJson["data"].(map[string]interface{}); ok {
					pair, ts := data["s"].(string), rJson["ts"].(float64)
					market, symbol := "usdt", strings.ToLower(pair[:len(pair)-len("USDT")])

					// Orderbook Update
					bids := parseOrders(data["b"].([]interface{}))
					asks := parseOrders(data["a"].([]interface{}))
					// isSnapshot := rJson["type"].(string) == "snapshot"
					updateOrderbook(&byfOrderbook, pair, bids, asks)

					// 현재 ask/bid 가격 출력
					bidPrice, askPrice := getBestPrices(&byfOrderbook, pair)
					bidPriceStr, askPriceStr := strconv.FormatFloat(bidPrice, 'f', -1, 64), strconv.FormatFloat(askPrice, 'f', -1, 64)
					tsStr := strconv.FormatFloat(ts, 'f', -1, 64)

					// fmt.Printf("%s|%s|%s|%s|%s|%s\n", exchange, market, symbol, askPriceStr, bidPriceStr, tsStr)

					key := fmt.Sprintf("%s:%s:%s", exchange, market, symbol)
					value := fmt.Sprintf("%s:%s:%s", askPriceStr, bidPriceStr, tsStr)
					syncMap.Store(key, value)
				}
			}
		}
	}
}

func calShortLongPremium(pairMap map[string][]string) {
	for {
		mutualPairs := commons.Intersect(pairMap["bif"], pairMap["byf"])
		log.Printf("%s <-> %s %d mutual pairs\n", "bif", "byf", len(mutualPairs))

		for _, shortE := range []string{"bif", "byf"} {
			for _, longE := range []string{"byf", "bif"} {
				if shortE == longE {
					continue
				}
				for _, pair := range mutualPairs {
					var pairInfo = strings.Split(pair, ":")
					market, symbol := strings.ToLower(pairInfo[0]), strings.ToLower(pairInfo[1])

					shortInfo, ok := syncMap.Load(fmt.Sprintf("%s:%s:%s", shortE, market, symbol))
					if !ok {
						log.Printf("RETRY SHORT %s:%s:%s\n", shortE, market, symbol)
						time.Sleep(time.Millisecond * 100)
						continue
					}
					longInfo, ok := syncMap.Load(fmt.Sprintf("%s:%s:%s", longE, market, symbol))
					if !ok {
						log.Printf("RETRY LONG %s:%s:%s\n", longE, market, symbol)
						time.Sleep(time.Millisecond * 100)
						continue
					}

					// Premium 계산(ask:bid:ts)
					shortPrice, shortTs := strings.Split(shortInfo.(string), ":")[1], strings.Split(shortInfo.(string), ":")[2]
					longPrice, longTs := strings.Split(longInfo.(string), ":")[0], strings.Split(longInfo.(string), ":")[2]
					shortPriceFloat, _ := strconv.ParseFloat(shortPrice, 64)
					longPriceFloat, _ := strconv.ParseFloat(longPrice, 64)
					// premium := commons.RoundToDecimal((longPriceFloat-shortPriceFloat)/shortPriceFloat, 5) * 100 // long/short - 1 이 아니라(short이 클 때 비싸게 파는거니까 음수가 됨)
					premium := commons.RoundToDecimal((shortPriceFloat-longPriceFloat)/longPriceFloat, 5) * 100 // short/long - 1 로 해야지 short이 커서 프리미엄이 양수가 됨

					key := fmt.Sprintf("sl:%s:%s:%s:%s", shortE, longE, market, symbol)
					value := fmt.Sprintf("%f:%s:%s", premium, shortTs, longTs)
					// msg := fmt.Sprintf("%s -> %.6s (short: %.8s, long: %.8s)\n", key, value, shortPrice, longPrice)

					// log.Printf("%s", msg)

					// if premium > 0.5 {
					// 	if premium < 1.0 {
					// 		tgmanager.SendMsg(msg)
					// 	}
					// }

					redismanager.PreHandlePremium(key, value)
					time.Sleep(1 * time.Millisecond)
				}
			}
		}
	}
}

// func calSpotFuturePremium(pairMap map[string][]string) {
// 	for {
// 		for _, se := range []string{"bin", "byb"} {
// 			for _, fe := range []string{"bif", "byf"} {
// 				mutualPairs := commons.Intersect(pairMap[se], pairMap[fe])

// 				for _, pair := range mutualPairs {
// 					var pairInfo = strings.Split(pair, ":")
// 					market, symbol := strings.ToLower(pairInfo[0]), strings.ToLower(pairInfo[1])

// 					spot, ok := syncMap.Load(fmt.Sprintf("%s:%s:%s", se, market, symbol))
// 					if !ok {
// 						log.Printf("RETRY SPOT %s:%s:%s\n", se, market, symbol)
// 						time.Sleep(time.Millisecond * 100)
// 						continue
// 					}

// 					future, ok := syncMap.Load(fmt.Sprintf("%s:%s:%s", fe, market, symbol))
// 					if !ok {
// 						log.Printf("RETRY FUTURE %s:%s:%s\n", fe, market, symbol)
// 						time.Sleep(time.Millisecond * 100)
// 						continue
// 					}

// 					// Premium 계산
// 					spotPrice, spotTs := strings.Split(spot.(string), ":")[0], strings.Split(spot.(string), ":")[2]
// 					futurePrice, futureTs := strings.Split(future.(string), ":")[1], strings.Split(future.(string), ":")[2]
// 					spotPriceFloat, _ := strconv.ParseFloat(spotPrice, 64)
// 					futurePriceFloat, _ := strconv.ParseFloat(futurePrice, 64)
// 					premium := commons.RoundToDecimal((futurePriceFloat-spotPriceFloat)/spotPriceFloat, 5) * 100

// 					key := fmt.Sprintf("sf:%s:%s:%s:%s", se, fe, market, symbol)
// 					value := fmt.Sprintf("%f:%s:%s", premium, spotTs, futureTs)
// 					// msg := fmt.Sprintf("%s -> %.6s (spot: %.8s, future: %.8s)\n", key, value, spotPrice, futurePrice)

// 					// log.Printf("%s", msg)

// 					// if premium > 0.5 {
// 					// 	if premium < 1.0 {
// 					// 		tgmanager.SendMsg(msg)
// 					// 	}
// 					// }

// 					redismanager.PreHandlePremium(key, value)
// 					time.Sleep(1 * time.Millisecond)
// 				}
// 			}
// 		}
// 	}
// }

func getPairMap() map[string][]string {
	pairMap := make(map[string][]string)
	// pairMap["bin"], pairMap["bif"] = binance.GetBinExchangeInfo(), binance.GetBifExchangeInfo()
	// pairMap["byb"], pairMap["byf"] = bybit.GetBybExchangeInfo(), bybit.GetByfExchangeInfo()

	pairMap["bif"], pairMap["byf"] = binance.GetBifExchangeInfo(), bybit.GetByfExchangeInfo()
	return pairMap
}

func Run() {
	var wg sync.WaitGroup
	var once sync.Once
	subscribeDone := make(chan struct{})

	pairMap := getPairMap()

	// pong 함수는 무한 루프에서 별도로 실행
	wg.Add(1)
	go func() {
		defer wg.Done()
		pong()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		subscribe(pairMap)                       // 구독 요청
		once.Do(func() { close(subscribeDone) }) // 구독 완료 신호
	}()
	<-subscribeDone // 구독 완료 신호 대기

	// subscribe 완료 후 receive 함수들 실행
	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	go receiveBin("bin")
	// }()
	wg.Add(1)
	go func() {
		defer wg.Done()
		go receiveBif("bif")
	}()
	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	go receiveByb("byb")
	// }()
	wg.Add(1)
	go func() {
		defer wg.Done()
		go receiveByf("byf")
	}()

	// 2개의 calculatePremium은 receive 함수 시작 후 바로 실행
	// go calSpotFuturePremium(pairMap)
	go calShortLongPremium(pairMap)

	wg.Wait()
}
