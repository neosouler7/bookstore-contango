package config

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"reflect"
)

var (
	errNotStruct = errors.New("config not struct")
	errNoField   = errors.New("field not found")
)

type config struct {
	Name  string
	Redis redis
	Tg    tg
}

type redis struct {
	Host string
	Port string
	Pwd  string
	Db   int
}

type tg struct {
	Token    string
	Chat_ids []int
}

func readConfig(obj interface{}, fieldName string) reflect.Value {
	s := reflect.ValueOf(obj).Elem()
	if s.Kind() != reflect.Struct {
		log.Fatalln(errNotStruct)
		// tgmanager.HandleErr("readConfig", errNotStruct)
	}
	f := s.FieldByName(fieldName)
	if !f.IsValid() {
		log.Fatalln(errNoField)
		// tgmanager.HandleErr("readConfig", errNoField)
	}
	return f
}

func getConfig(key string) interface{} {
	path, _ := os.Getwd()
	file, _ := os.Open(path + "/config/config.json")
	defer file.Close()

	c := config{}
	err := json.NewDecoder(file).Decode(&c)
	if err != nil {
		log.Fatalln(err)
	}
	// tgmanager.HandleErr("getConfig", json.NewDecoder(file).Decode(&c))
	return readConfig(&c, key).Interface()
}

func GetName() string {
	return getConfig("Name").(string)
}

func GetRedis() redis {
	return getConfig("Redis").(redis)
}

func GetTg() tg {
	return getConfig("Tg").(tg)
}
