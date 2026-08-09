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
	"github.com/Ak-Army/config/crypto"
	"github.com/Ak-Army/config/crypto/aesgcm"
	"github.com/Ak-Army/xlog"
)

var defaults reflect.Value

type Config struct {
	mu       sync.RWMutex
	Defaults *BlockInfo `config:"defaults"`
	Blocks   []Block    `config:"blocks"`

	bar *Bar
}

func (c *Config) Default() *Config {
	xlog.Info("New snapshot")
	return &Config{}
}

func (c *Config) Set(conf *Config) {
	go c.apply(conf)
}

func (c *Config) apply(conf *Config) {
	// Nothing above this goroutine can recover for us, so a bad config must not
	// be able to take the process down.
	defer func() {
		if r := recover(); r != nil {
			xlog.Errorf("Unable to apply the configuration: %v %s", r, debug.Stack())
		}
	}()
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Defaults = conf.Defaults
	c.Blocks = conf.Blocks

	// Build first, swap second: createBar recovers from a panic and returns nil,
	// and a broken config should leave the running bar alone rather than replace
	// it with nothing.
	bar := conf.createBar()
	if bar == nil {
		xlog.Error("Configuration was not applied, keeping the previous bar")
		return
	}
	hasBar := c.bar != nil
	if hasBar {
		c.bar.Stop()
	}
	c.bar = bar
	if !hasBar {
		c.bar.Start()
	} else {
		c.bar.ReStart()
	}
}

func New(f string, keyring string) error {
	cr, cerr := crypto.New(keyring,
		func(key []byte) (crypto.Decrypter, error) {
			return aesgcm.New(key)
		})
	if cerr != nil {
		xlog.Warnf("Unable to read the config keyring, encrypted values will fail: %s", cerr)
	}
	loader, err := config.NewLoader(context.Background(),
		file.New(
			file.WithPath(f),
			file.WithWatchInterval(time.Minute),
			file.WithOption(backend.WithWatcher()),
		),
	)
	if err != nil {
		return err
	}
	if cr != nil {
		loader.SetCrypto(cr)
	}
	conf := config.NewStore[Config](&Config{})
	return config.Load(loader, conf)
}

func (c *Config) createBar() *Bar {
	defer func() {
		if err := recover(); err != nil {
			xlog.Errorf("%+v %s", err, string(debug.Stack()))
		}
	}()
	log := xlog.GetLogger()
	updateChannel := make(chan UpdateChannelMsg)
	stop := make(chan bool)
	// The `defaults` section is optional. Assign either way, so a reload that
	// drops the section does not keep applying the previous one.
	defaults = defaultsValueFor(c.Defaults)
	for i := range c.Blocks {
		mapDefaults(&c.Blocks[i].Info)
		err := c.Blocks[i].CreateModule(i, log)
		xlog.Info("Block create", c.Blocks[i].Info.Name)
		if err == nil {
			// The goroutine runs on its own copy: Bar.blocks below shares this
			// backing array, so starting it on &c.Blocks[i] would have the block
			// read and write the very element Bar.Print and Bar.update touch
			// under the bar's mutex. State reaches the bar through updateChannel.
			block := c.Blocks[i]
			go block.Start(i, updateChannel, stop)
		} else {
			log.Error(err)
		}
	}

	log.Infof("Bar items: %+v", c.Blocks)
	return &Bar{
		blocks:        c.Blocks,
		log:           log,
		updateChannel: updateChannel,
		stop:          stop,
	}
}

// defaultsValueFor turns the optional `defaults` section into the value
// mapDefaults reads. A missing section yields the zero Value, which mapDefaults
// skips rather than panicking on.
func defaultsValueFor(info *BlockInfo) reflect.Value {
	if info == nil {
		return reflect.Value{}
	}
	return reflect.ValueOf(info).Elem()
}

func mapDefaults(blockInfo *BlockInfo) {
	if !defaults.IsValid() {
		return
	}
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
