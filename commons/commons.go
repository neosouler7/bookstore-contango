package commons

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/neosouler7/bookstore-contango/tgmanager"
)

func GetObTargetPrice(volume string, orderbook interface{}) string {
	/*
		ask's price should go up, and bid should go down

		ask = [[p1, v1], [p2, v2], [p3, v3] ...]
		bid = [[p3, v3], [p2, v2], [p1, p1] ...]
	*/
	currentVolume := 0.0
	targetVolume, err := strconv.ParseFloat(volume, 64)
	tgmanager.HandleErr("GetObTargetPrice1", err)

	obSlice := orderbook.([]interface{})
	for _, ob := range obSlice {
		obInfo := ob.([2]string)
		volume, err := strconv.ParseFloat(obInfo[1], 64)
		tgmanager.HandleErr("GetObTargetPrice2", err)

		currentVolume += volume
		if currentVolume >= targetVolume {
			return obInfo[0]
		}
	}
	return obSlice[len(obSlice)-1].([2]string)[0]
}

func FormatTs(ts string) string {
	if len(ts) < 13 {
		add := strings.Repeat("0", 13-len(ts))
		return fmt.Sprintf("%s%s", ts, add)
	} else if len(ts) == 13 { // if millisecond
		return ts
	} else {
		return ts[:13]
	}
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Bytes2Json(data []byte, i interface{}) {
	r := bytes.NewReader(data)
	err := json.NewDecoder(r).Decode(i)
	tgmanager.HandleErr("Bytes2Json", err)
}

func SetTimeZone(name string) *time.Location {
	tz := os.Getenv("TZ")
	if tz == "" {
		tz = "Asia/Seoul"
		fmt.Printf("%s : DEFAULT %s\n", name, tz)
	} else {
		fmt.Printf("%s : SERVER %s\n", name, tz)
	}
	location, _ := time.LoadLocation(tz)
	return location
}

func RoundToDecimal(value float64, n int) float64 {
	pow := math.Pow(10, float64(n))
	return math.Round(value*pow) / pow
}

func Intersect(slice1, slice2 []string) []string {
	lookup := make(map[string]struct{}) // 첫 번째 슬라이스 요소를 map에 저장
	for _, item := range slice1 {
		lookup[item] = struct{}{}
	}

	var result []string // 공통된 요소를 저장할 결과 슬라이스
	for _, item := range slice2 {
		if _, exists := lookup[item]; exists {
			result = append(result, item)
			delete(lookup, item) // 중복 방지를 위해 제거
		}
	}
	return result
}
