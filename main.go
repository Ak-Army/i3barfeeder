package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/Ak-Army/i3barfeeder/gobar"
	_ "github.com/Ak-Army/i3barfeeder/modules"

	"github.com/Ak-Army/xlog"
)

func loadConfig(path string) (barConfig gobar.Config, err error) {
	barConfig = gobar.Config{}
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

func main() {
	var logPath, configPath string
	flag.StringVar(&logPath, "log", "/dev/null", "Log path. Default: /dev/null")
	flag.StringVar(&logPath, "l", "/dev/null", "Log file to use. Default: /dev/null")
	flag.StringVar(&configPath, "config", "", "Config path.")
	flag.StringVar(&configPath, "c", "", "Config path (in JSON).")

	flag.Parse()

	logfile, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to open log file: %q", err)
		os.Exit(2)
	}
	defer logfile.Close()

	log := xlog.New(xlog.Config{
		Output: xlog.NewLogfmtOutput(logfile),
	})
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Unhandled panic: %v", r)
		}
	}()
	log.Info("Start")
	log.Infof("Loading configuration from: %s", configPath)
	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatal("Unable to load config", err)
	}
	log.Infof("bar items: %+v", config.Blocks)

	bar := config.CreateBar(log)
	bar.Start()
	log.Info("End")
	os.Exit(0)

}
