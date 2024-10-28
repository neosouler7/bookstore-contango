package binance

import (
	"fmt"
	"strings"

	"github.com/neosouler7/bookstore-contango/restmanager"
)

const (
	bin string = "https://api.binance.com"
	bif string = "https://fapi.binance.com"
	GET        = "GET"
	PUT        = "PUT"
)

func GetBinExchangeInfo() []string {
	endPoint := bin + "/api/v3/exchangeInfo"
	queryString := ""

	c := make(chan map[string]interface{})
	go restmanager.FastHttpRequest(c, "bin", GET, endPoint, queryString)
	rJson := <-c

	var symbolSlice []string
	for _, s := range rJson["symbols"].([]interface{}) {
		market, symbol := s.(map[string]interface{})["quoteAsset"].(string), s.(map[string]interface{})["baseAsset"].(string)
		if s.(map[string]interface{})["status"].(string) == "TRADING" && market == "USDT" {
			symbolSlice = append(symbolSlice, fmt.Sprintf("%s:%s", strings.ToLower(market), strings.ToLower(symbol)))
		}
	}
	return symbolSlice
}
func GetBifExchangeInfo() []string {
	endPoint := bif + "/fapi/v1/exchangeInfo"
	queryString := ""

	c := make(chan map[string]interface{})
	go restmanager.FastHttpRequest(c, "bin", GET, endPoint, queryString)
	rJson := <-c

	var symbolSlice []string
	for _, s := range rJson["symbols"].([]interface{}) {
		market, symbol := s.(map[string]interface{})["quoteAsset"].(string), s.(map[string]interface{})["baseAsset"].(string)
		if s.(map[string]interface{})["status"].(string) == "TRADING" && market == "USDT" {
			symbolSlice = append(symbolSlice, fmt.Sprintf("%s:%s", strings.ToLower(market), strings.ToLower(symbol)))
		}
	}
	return symbolSlice

}
