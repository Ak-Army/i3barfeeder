package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"reflect"

	"github.com/Ak-Army/i3barfeeder/gobar"
	"github.com/Ak-Army/i3barfeeder/modules"
)

func loadConfig(path string) (barConfig gobar.Configuration, err error) {
	barConfig = gobar.Configuration{}
	text, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	err = json.Unmarshal(text, &barConfig)
	return
}

func checkErr(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s:%q", msg, err)
		os.Exit(2)
	}
}

func initModules() {
	gobar.AddModule("StaticText", reflect.TypeOf(modules.StaticText{}))
	gobar.AddModule("DateTime", reflect.TypeOf(modules.DateTime{}))
	gobar.AddModule("ExternalCmd", reflect.TypeOf(modules.ExternalCmd{}))
	gobar.AddModule("DiskUsage", reflect.TypeOf(modules.DiskUsage{}))
	gobar.AddModule("MemInfo", reflect.TypeOf(modules.MemInfo{}))
	gobar.AddModule("CpuInfo", reflect.TypeOf(modules.CpuInfo{}))
	gobar.AddModule("VolumeInfo", reflect.TypeOf(modules.VolumeInfo{}))
	gobar.AddModule("Toggl", reflect.TypeOf(modules.Toggl{}))
}

func main() {
	var logPath, configPath string
	flag.StringVar(&logPath, "log", "/dev/null", "Log path. Default: /dev/null")
	flag.StringVar(&logPath, "l", "/dev/null", "Log file to use. Default: /dev/null")
	flag.StringVar(&configPath, "config", "", "Configuration path.")
	flag.StringVar(&configPath, "c", "", "Configuration path (in JSON).")

	flag.Parse()

	logfile, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	checkErr(err, "Log file not opened")
	defer logfile.Close()
	logger := log.New(logfile, "gobar:", log.Lshortfile|log.LstdFlags)

	logger.Println("Start")
	initModules()
	logger.Printf("Loading configuration from: %s", configPath)
	config, err := loadConfig(configPath)
	checkErr(err, "Config file not opened")
	logger.Printf("bar items: %+v", config.Blocks)

	bar := config.CreateBar(logger)
	bar.Start()
	sigHandler(bar, logger)
	logger.Println("End")
	os.Exit(0)
	defer func () {
		if r := recover(); r != nil {
			logger.Printf("Unhandled panic: %v", r)
		}
	}()
}

func sigHandler(bar *gobar.Bar, logger *log.Logger) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	for {
		sig := <-sigs
		logger.Printf("Received signal: %q", sig)
		switch sig {
		/*case syscall.SIGSTOP:
			bar.Stop()
		case syscall.SIGCONT:
			bar.Stop()
			bar.ReStart()*/
		case syscall.SIGINT:
			return
		}
	}
}
