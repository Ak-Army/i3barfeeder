package gobar

import (
	"context"
	"reflect"
	"runtime/debug"
	"sync"
	"time"

	"github.com/Ak-Army/config"
	"github.com/Ak-Army/config/backend"
	"github.com/Ak-Army/config/backend/file"
	"github.com/Ak-Army/xlog"
)

var store *Store
var defaults reflect.Value

type Config struct {
	Defaults *BlockInfo `config:"defaults"`
	Blocks   []Block    `config:"blocks"`
}

type Store struct {
	mu     sync.RWMutex
	config *Config
	bar    *Bar
	err    error
}

func (c *Store) NewSnapshot() interface{} {
	xlog.Info("New snapshot")
	return &Config{}
}

func (c *Store) SetSnapshot(confInterface interface{}, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	conf := confInterface.(*Config)
	c.config = conf
	if c.bar != nil {
		newBar := c.config.createBar()
		c.bar.Stop()
		c.bar = newBar
		c.bar.ReStart()
	}
	c.err = err
}

func (c *Store) Config() (*Config, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config, c.err
}

func New(f string) (*Store, error) {
	var err error
	(&sync.Once{}).Do(func() {
		var loader *config.Loader
		store = &Store{}
		loader, store.err = config.NewLoader(context.Background(),
			file.New(
				file.WithPath(f),
				file.WithWatchInterval(time.Minute),
				file.WithOption(backend.WithWatcher()),
			),
		)
		if err != nil {
			return
		}
		store.err = loader.Load(store)
	})
	return store, err
}

func (c *Store) Start() {
	c.bar = c.config.createBar()
	c.bar.Start()
}

func (c *Config) createBar() *Bar {
	defer func() {
		if err := recover(); err != nil {
			xlog.Errorf("%+v %s", err, string(debug.Stack()))
		}
	}()
	log := xlog.GetLogger()
	updateChannel := make(chan UpdateChannelMsg)
	xlog.Info(c.Defaults)
	defaults = reflect.ValueOf(c.Defaults).Elem()
	for i := range c.Blocks {
		mapDefaults(&c.Blocks[i].Info)
		err := c.Blocks[i].CreateModule(i, log)
		if err == nil {
			go c.Blocks[i].Start(i, updateChannel)
		} else {
			log.Error(err)
		}
	}

	log.Infof("Bar items: %+v", c.Blocks)
	return &Bar{
		blocks:        c.Blocks,
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
