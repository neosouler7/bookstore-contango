package bybit

import (
	"fmt"
	"strings"

	"github.com/neosouler7/bookstore-contango/restmanager"
)

const (
	byb string = "https://api.bybit.com"
	byf string = "https://api.bybit.com"
	GET        = "GET"
	PUT        = "PUT"
)

func GetBybExchangeInfo() []string {
	endPoint := byb + "/v5/market/instruments-info?"
	queryString := "category=spot"

	c := make(chan map[string]interface{})
	go restmanager.FastHttpRequest(c, "byb", GET, endPoint, queryString)
	rJson := <-c

	var symbolSlice []string
	for _, s := range rJson["result"].(map[string]interface{})["list"].([]interface{}) {
		market, symbol := s.(map[string]interface{})["quoteCoin"].(string), s.(map[string]interface{})["baseCoin"].(string)
		if s.(map[string]interface{})["status"].(string) == "Trading" && market == "USDT" {
			symbolSlice = append(symbolSlice, fmt.Sprintf("%s:%s", strings.ToLower(market), strings.ToLower(symbol)))
		}
	}
	return symbolSlice
}

func GetByfExchangeInfo() []string {
	endPoint := byf + "/v5/market/instruments-info?"
	queryString := "category=linear"

	c := make(chan map[string]interface{})
	go restmanager.FastHttpRequest(c, "byf", GET, endPoint, queryString)
	rJson := <-c

	var symbolSlice []string
	for _, s := range rJson["result"].(map[string]interface{})["list"].([]interface{}) {
		market, symbol := s.(map[string]interface{})["quoteCoin"].(string), s.(map[string]interface{})["baseCoin"].(string)
		if s.(map[string]interface{})["status"].(string) == "Trading" && market == "USDT" {
			symbolSlice = append(symbolSlice, fmt.Sprintf("%s:%s", strings.ToLower(market), strings.ToLower(symbol)))
		}
	}
	return symbolSlice
}
