package gobar

import (
	"encoding/json"
	"fmt"
	"strings"
	"log"
	"time"
	"bufio"
	"os"
)

// Header i3  header
type Header struct {
	Version     int  `json:"version"`
	ClickEvents bool `json:"click_events"`
	// StopSignal     int  `json:"stop_signal"`
	// ContinueSignal int  `json:"cont_signal"`
}

type Bar struct {
	modules       []Block
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
	header := Header{
		Version:     1,
		ClickEvents: true,
		// StopSignal:     20, // SIGHUP
		// ContinueSignal: 19, // SIGCONT
	}
	headerJSON, _ := json.Marshal(header)
	fmt.Println(string(headerJSON))
	fmt.Println("[[]")
	bar.stop = make(chan bool, 3)
	bar.ReStart()
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

func (bar *Bar) update() {
	for {
		select {
		case <-bar.stop:
			bar.logger.Println("update")
			return
		case m := <-bar.updateChannel:
			bar.modules[m.ID].Info = m.Info
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
				for _, module := range bar.modules {
					if clickMessage.isMatch(module) {
						bar.logger.Println("Click: handled")
						err := module.HandleClick(clickMessage)
						if err != nil {
							bar.logger.Println("Click handle error: %s", err.Error())
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
			var infoArray []string
			var minInterval int64
			for _, item := range bar.modules {
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
			if minInterval == 0 {
				break;
			}
			time.Sleep(time.Duration(minInterval) * time.Second)
		}
	}
}
