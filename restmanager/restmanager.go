package restmanager

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/neosouler7/bookstore-contango/commons"
	"github.com/neosouler7/bookstore-contango/tgmanager"

	"github.com/valyala/fasthttp"
)

var (
	c    *fasthttp.Client
	once sync.Once
)

const (
	bin string = "https://api.binance.com"
	bif string = "https://fapi.binance.com"
)

type epqs struct {
	endPoint    string
	queryString string
}

// func (e *epqs) getEpqs(exchange, market, symbol string) {
// 	switch exchange {
// 	case "bin":
// 		e.endPoint = bin + "/api/v3/depth"
// 		e.queryString = fmt.Sprintf("limit=50&symbol=%s%s", strings.ToUpper(symbol), strings.ToUpper(market))
// 	case "bif":
// 		e.endPoint = bif + "/fapi/v1/depth"
// 		e.queryString = fmt.Sprintf("limit=50&symbol=%s%s", strings.ToUpper(symbol), strings.ToUpper(market))
// 	}
// }

func fastHttpClient() *fasthttp.Client {
	once.Do(func() {
		clientPointer := &fasthttp.Client{}
		c = clientPointer
	})
	return c
}

func FastHttpRequest(c chan<- map[string]interface{}, exchange, method, endPoint, queryString string) {
	req, res := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	defer func() {
		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(res)
	}()

	req.Header.SetMethod(method)
	req.SetRequestURI(endPoint)
	req.URI().SetQueryString(queryString)
	statusCode, body, err := fastHttpClient().GetTimeout(nil, req.URI().String(), time.Duration(5)*time.Second)

	tgmanager.HandleErr(exchange, err)
	if len(body) == 0 {
		errHttpResponseBody := errors.New("empty response body")
		tgmanager.HandleErr(exchange, errHttpResponseBody)
	}
	if statusCode != fasthttp.StatusOK {
		errHttpResponseStatus := fmt.Errorf("restapi error with status %d", statusCode)
		tgmanager.HandleErr(exchange, errHttpResponseStatus)
	}

	var rJson interface{}
	commons.Bytes2Json(body, &rJson)
	c <- rJson.(map[string]interface{})
	// switch exchange {
	// case "bin":
	// 	var rJson interface{}
	// 	commons.Bytes2Json(body, &rJson)

	// 	c <- rJson.(map[string]interface{})

	// 	fmt.Println("55")
	// case "bif":
	// 	var rJson interface{}
	// 	commons.Bytes2Json(body, &rJson)

	// 	c <- rJson.(map[string]interface{})
	// }
}
