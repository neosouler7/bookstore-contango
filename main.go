package main

import (
	"fmt"

	_ "net/http/pprof"

	"github.com/neosouler7/bookstore-contango/commons"
	"github.com/neosouler7/bookstore-contango/config"
	"github.com/neosouler7/bookstore-contango/contango"
	"github.com/neosouler7/bookstore-contango/tgmanager"
)

func main() {
	// only for pprof
	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	// for profiling
	// go func() {
	// 	for {
	// 		var m runtime.MemStats
	// 		runtime.ReadMemStats(&m)
	// 		fmt.Printf("HeapAlloc = %v", (m.HeapAlloc))
	// 		fmt.Printf("\tHeapObjects = %v", (m.HeapObjects))
	// 		fmt.Printf("\tHeapSys = %v", (m.Sys))
	// 		fmt.Printf("\tNumGC = %v\n", m.NumGC)
	// 		time.Sleep(5 * time.Second)
	// 	}
	// }()

	tgConfig := config.GetTg()
	tgmanager.InitBot(
		tgConfig.Token,
		tgConfig.Chat_ids,
		commons.SetTimeZone("Tg"),
	)

	tgMsg := fmt.Sprintf("## START %s", config.GetName())
	tgmanager.SendMsg(tgMsg)

	contango.Run()
}
