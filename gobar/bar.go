package gobar

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Header i3  header
type header struct {
	Version     int  `json:"version"`
	ClickEvents bool `json:"click_events"`
	//StopSignal     syscall.Signal  `json:"stop_signal"`
	//ContinueSignal syscall.Signal  `json:"cont_signal"`
}

type Bar struct {
	blocks        []Block
	logger        *log.Logger
	updateChannel chan UpdateChannelMsg
	stop          chan bool
}

type ClickMessage struct {
	Name     string `json:"name,omitempty"`
	Instance string `json:"instance,omitempty"`
	Button   int    `json:"button"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
}

func (cm *ClickMessage) isMatch(block Block) bool {
	return block.Info.Name == cm.Name && block.Info.Instance == cm.Instance
}

func (bar *Bar) Start() {
	header := header{
		Version:     1,
		ClickEvents: true,
		//StopSignal:     syscall.SIGTERM,
		//ContinueSignal: syscall.SIGCONT,
	}
	headerJSON, _ := json.Marshal(header)
	fmt.Println(string(headerJSON))
	fmt.Println("[[]")
	bar.stop = make(chan bool, 3)
	bar.ReStart()
	bar.sigHandler()
}

func (bar *Bar) ReStart() {
	bar.stop = make(chan bool, 3)
	go bar.update()
	go bar.printItems()
	go bar.handleClick()
}

func (bar *Bar) Stop() {
	bar.stop <- true
	bar.stop <- true
	bar.stop <- true
}

func (bar *Bar) Print() (minInterval int64) {
	var infoArray []string
	for _, item := range bar.blocks {
		item.Info.FullText = item.Label + " " + item.Info.FullText
		item.Info.ShortText = item.Label + " " + item.Info.ShortText

		info, err := json.Marshal(item.Info)
		if err != nil {
			bar.logger.Printf("ERROR: %q", err)
		} else {
			infoArray = append(infoArray, string(info))
		}
		if minInterval == 0 || (item.Interval > 0 && item.Interval < minInterval) {
			minInterval = item.Interval
		}
	}
	fmt.Println(",[", strings.Join(infoArray, ",\n"), "]")

	return minInterval
}

func (bar *Bar) sigHandler() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGCONT)
	for {
		sig := <-sigs
		bar.logger.Printf("Received signal: %q", sig)
		switch sig {
		/*case syscall.SIGTERM:
			bar.Stop()
		case syscall.SIGCONT:
			bar.Stop()
			bar.ReStart()*/
		case syscall.SIGINT:
			return
		}
	}
}

func (bar *Bar) update() {
	for {
		select {
		case <-bar.stop:
			bar.logger.Println("update")
			return
		case m := <-bar.updateChannel:
			bar.blocks[m.ID].Info = m.Info
		}
	}
}

func (bar *Bar) handleClick() {
	for {
		select {
		case <-bar.stop:
			bar.logger.Println("handleClick")
			return
		default:
			bio := bufio.NewReader(os.Stdin)
			line, _, err := bio.ReadLine()
			if err != nil {
				continue
			}
			if len(line) == 0 {
				continue
			}

			var clickMessage ClickMessage

			if line[0] == ',' {
				line = line[1:]
			}

			err = json.Unmarshal(line, &clickMessage)
			if err == nil {
				bar.logger.Printf("Click: line: %s, cm:%+v", string(line), clickMessage)
				for i, block := range bar.blocks {
					if clickMessage.isMatch(block) {
						bar.logger.Println("Click: handled")
						info, err := block.HandleClick(clickMessage)
						if err != nil {
							bar.logger.Println("Click handle error: %s", err.Error())
						}
						if info != nil {
							bar.blocks[i].Info = *info
							bar.Print()
						}
					}
				}
			}
			time.Sleep(1)
		}
	}
}

func (bar *Bar) printItems() {
	for {
		select {
		case <-bar.stop:
			bar.logger.Println("printItems")
			return
		default:
			minInterval := bar.Print()
			if minInterval == 0 {
				break
			}
			time.Sleep(time.Duration(minInterval) * time.Second)
		}
	}
}
