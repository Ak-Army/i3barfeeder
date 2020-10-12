package gobar

import (
	"reflect"

	"github.com/Ak-Army/xlog"
)

type Config struct {
	Defaults *BlockInfo `json:"defaults,omitempty"`
	Blocks   []Block    `json:"blocks"`
}

var defaults reflect.Value

func (config *Config) CreateBar(log xlog.Logger) *Bar {
	// fmt.Printf("%+v\n\n", reflect.TypeOf(config.Defaults).Elem().Field(0))
	updateChannel := make(chan UpdateChannelMsg)
	defaults = reflect.ValueOf(config.Defaults).Elem()
	for i := range config.Blocks {
		mapDefaults(&config.Blocks[i].Info)
		err := config.Blocks[i].CreateModule(i, log)
		if err == nil {
			go config.Blocks[i].Start(i, updateChannel)
		} else {
			log.Error(err)
		}
	}
	return &Bar{
		blocks:        config.Blocks,
		log:           log,
		updateChannel: updateChannel,
	}
}

func mapDefaults(blockInfo *BlockInfo) {
	info := reflect.ValueOf(blockInfo).Elem()

	for i, n := 0, defaults.NumField(); i < n; i++ {
		src := defaults.Field(i)
		dst := info.Field(i)
		if !isEmptyValue(src) && isEmptyValue(dst) && dst.CanSet() {
			dst.Set(src)
		}
	}
}

// From src/pkg/encoding/json.
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}
