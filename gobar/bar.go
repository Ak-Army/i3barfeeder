package gobar

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"sync"
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
	stopOnce      sync.Once
	mu            sync.Mutex
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
	go b.update()
	go b.printItems()
	go b.handleClick()
}

func (b *Bar) Stop() {
	if b.stop == nil {
		return
	}
	b.stopOnce.Do(func() {
		close(b.stop)
		b.mu.Lock()
		blocks := make([]Block, len(b.blocks))
		copy(blocks, b.blocks)
		b.mu.Unlock()
		for i := range blocks {
			blocks[i].Stop()
		}
	})
}

func (b *Bar) Print() (minInterval int64) {
	line, minInterval := b.render()
	fmt.Println(line)

	return minInterval
}

// render builds one line of the i3bar stream and reports the shortest interval
// among the blocks. Splitting it from Print keeps the formatting testable.
func (b *Bar) render() (line string, minInterval int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var infoArray []string
	for _, item := range b.blocks {
		if item.Label != "" {
			item.Info.FullText = item.Label + " " + item.Info.FullText
			item.Info.ShortText = item.Label + " " + item.Info.ShortText
		}

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

	return ",[ " + strings.Join(infoArray, ",\n") + " ]", minInterval
}

func (b *Bar) update() {
	for {
		select {
		case <-b.stop:
			b.log.Debug("Stop update")
			return
		case m := <-b.updateChannel:
			b.mu.Lock()
			b.blocks[m.ID].Info = m.Info
			b.mu.Unlock()
		}
	}
}

var (
	clickOnce     sync.Once
	clickMessages chan ClickMessage
)

// clickChannel reads i3bar's click events for the lifetime of the process. It
// is deliberately not per-bar: bufio reads ahead, so a second reader started by
// a config reload would swallow events buffered by the first one.
func clickChannel() <-chan ClickMessage {
	clickOnce.Do(func() {
		clickMessages = make(chan ClickMessage, 16)
		go readClicks(os.Stdin, clickMessages)
	})
	return clickMessages
}

func readClicks(r io.Reader, out chan<- ClickMessage) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		clickMessage, ok := parseClick(scanner.Bytes())
		if !ok {
			continue
		}
		xlog.Debugf("Click: cm:%+v", clickMessage)
		out <- clickMessage
	}
	if err := scanner.Err(); err != nil {
		xlog.Warn("Stdin scan error", err)
	} else {
		xlog.Debug("Stdin closed, no more click events")
	}
}

// parseClick decodes one line of the click event stream. Every line but the
// first is prefixed with a comma, which is not part of the JSON.
func parseClick(line []byte) (ClickMessage, bool) {
	var clickMessage ClickMessage
	if len(line) == 0 {
		return clickMessage, false
	}
	if line[0] == ',' {
		line = line[1:]
	}
	if err := json.Unmarshal(line, &clickMessage); err != nil {
		return clickMessage, false
	}
	return clickMessage, true
}

func (b *Bar) handleClick() {
	clicks := clickChannel()
	for {
		select {
		case <-b.stop:
			b.log.Debug("Stop handleClick")
			return
		case cm := <-clicks:
			b.handleClickMessage(cm)
		}
	}
}

func (b *Bar) handleClickMessage(clickMessage ClickMessage) {
	b.mu.Lock()
	var matched []int
	for i, block := range b.blocks {
		if clickMessage.isMatch(block) {
			matched = append(matched, i)
		}
	}
	blocks := make([]Block, len(matched))
	for j, i := range matched {
		blocks[j] = b.blocks[i]
	}
	b.mu.Unlock()

	var changed bool
	for j, i := range matched {
		b.log.Debug("Click: handled")
		info, err := b.clickBlock(&blocks[j], clickMessage)
		if err != nil {
			b.log.Debug("Click: error: ", err.Error())
		}
		if info != nil {
			b.mu.Lock()
			b.blocks[i].Info = *info
			b.mu.Unlock()
			changed = true
		}
	}
	if changed {
		b.Print()
	}
}

func (b *Bar) clickBlock(block *Block, clickMessage ClickMessage) (info *BlockInfo, err error) {
	defer func() {
		if r := recover(); r != nil {
			b.log.Errorf("recovered: %s -> stackTrace: %s", r, debug.Stack())
			info, err = nil, fmt.Errorf("click handler panicked: %v", r)
		}
	}()
	return block.HandleClick(clickMessage)
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
				b.log.Debug("No interval, stop printItems")
				return
			}
			time.Sleep(time.Duration(minInterval) * time.Second)
		}
	}
}
