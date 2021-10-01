package gobar

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Ak-Army/xlog"
)

// Header i3  header
type header struct {
	Version     int  `json:"version"`
	ClickEvents bool `json:"click_events"`
	// StopSignal     syscall.Signal  `json:"stop_signal"`
	// ContinueSignal syscall.Signal  `json:"cont_signal"`
}

type Bar struct {
	blocks        []Block
	log           xlog.Logger
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

func (b *Bar) Start() {
	header := header{
		Version:     1,
		ClickEvents: true,
		// StopSignal:     syscall.SIGTERM,
		// ContinueSignal: syscall.SIGCONT,
	}
	headerJSON, _ := json.Marshal(header)
	fmt.Println(string(headerJSON))
	fmt.Println("[[]")
	b.ReStart()
}

func (b *Bar) ReStart() {
	b.stop = make(chan bool)
	go b.update()
	go b.printItems()
	go b.handleClick()
	<-b.stop
}

func (b *Bar) Stop() {
	close(b.stop)
}

func (b *Bar) Print() (minInterval int64) {
	var infoArray []string
	for _, item := range b.blocks {
		item.Info.FullText = item.Label + " " + item.Info.FullText
		item.Info.ShortText = item.Label + " " + item.Info.ShortText

		info, err := json.Marshal(item.Info)
		if err != nil {
			b.log.Error("ERROR: %q", err)
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

func (b *Bar) update() {
	for {
		select {
		case <-b.stop:
			b.log.Debug("Stop update")
			return
		case m := <-b.updateChannel:
			b.blocks[m.ID].Info = m.Info
		}
	}
}

func (b *Bar) handleClick() {
	for {
		select {
		case <-b.stop:
			b.log.Debug("Stop handleClick")
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
				b.log.Debugf("Click: line: %s, cm:%+v", string(line), clickMessage)
				for i, block := range b.blocks {
					if clickMessage.isMatch(block) {
						b.log.Debug("Click: handled")
						info, err := block.HandleClick(clickMessage)
						if err != nil {
							b.log.Debug("Click: error: ", err.Error())
						}
						if info != nil {
							b.blocks[i].Info = *info
							b.Print()
						}
					}
				}
			}
		}
	}
}

func (b *Bar) printItems() {
	for {
		select {
		case <-b.stop:
			b.log.Debug("Stop printItems")
			return
		default:
			minInterval := b.Print()
			if minInterval == 0 {
				break
			}
			time.Sleep(time.Duration(minInterval) * time.Second)
		}
	}
}
