package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Ak-Army/i3barfeeder/gobar"
	_ "github.com/Ak-Army/i3barfeeder/modules"

	"github.com/Ak-Army/xlog"
)

func main() {
	var logPath, configPath, keyring string
	flag.StringVar(&logPath, "log", "/dev/null", "Log path. Default: /dev/null")
	flag.StringVar(&logPath, "l", "/dev/null", "Log file to use. Default: /dev/null")
	flag.StringVar(&configPath, "config", "", "Config path.")
	flag.StringVar(&configPath, "c", "", "Config path (in JSON).")
	flag.StringVar(&keyring, "key", "", "Keyring filepath path.")
	flag.StringVar(&keyring, "k", "", "Keyring filepath path.")

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
	xlog.SetLogger(log)
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Unhandled panic: %v", r)
		}
	}()
	log.Info("Start")
	log.Infof("Loading configuration from: %s", configPath)
	if err := gobar.New(configPath, keyring); err != nil {
		log.Fatal("Unable to load config", err)
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGCONT)
	for {
		sig := <-sigs
		log.Debugf("Received signal: %q", sig)
		switch sig {
		/*case syscall.SIGCONT:
		bar.Stop()
		bar.ReStart()*/
		case syscall.SIGINT, syscall.SIGTERM:
			log.Info("End")
			return
		}
	}
}
