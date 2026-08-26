package gobar

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Ak-Army/xlog"
)

func TestMain(m *testing.M) {
	// The package logs through the global logger; keep the test output readable.
	xlog.SetLogger(xlog.NopLogger)
	os.Exit(m.Run())
}

func TestParseClick(t *testing.T) {
	tests := []struct {
		name string
		line string
		want ClickMessage
		ok   bool
	}{
		{
			name: "first event of the stream",
			line: `{"name":"VolumeInfo","instance":"id_1","button":5,"x":29,"y":12}`,
			want: ClickMessage{Name: "VolumeInfo", Instance: "id_1", Button: 5, X: 29, Y: 12},
			ok:   true,
		},
		{
			// i3bar prefixes every line after the first with a comma.
			name: "comma prefixed event",
			line: `,{"name":"DateTime","instance":"id_0","button":1,"x":0,"y":0}`,
			want: ClickMessage{Name: "DateTime", Instance: "id_0", Button: 1},
			ok:   true,
		},
		{name: "opening bracket of the stream", line: "[", ok: false},
		{name: "empty line", line: "", ok: false},
		{name: "lone comma", line: ",", ok: false},
		{name: "not json", line: "garbage", ok: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseClick([]byte(tc.line))
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestReadClicksDeliversEveryEvent(t *testing.T) {
	const events = 30
	var input strings.Builder
	input.WriteString(`{"name":"M","instance":"id_0","button":1,"x":0,"y":0}` + "\n")
	for i := 1; i < events; i++ {
		input.WriteString(`,{"name":"M","instance":"id_0","button":1,"x":0,"y":0}` + "\n")
	}

	out := make(chan ClickMessage, events)
	readClicks(strings.NewReader(input.String()), out)
	close(out)

	var got int
	for range out {
		got++
	}
	if got != events {
		t.Errorf("delivered %d events, want %d", got, events)
	}
}

func TestRender(t *testing.T) {
	bar := &Bar{
		log: xlog.NopLogger,
		blocks: []Block{
			{Label: "C:", Interval: 5, Info: BlockInfo{FullText: "50 %", ShortText: "50", Name: "CpuInfo"}},
			{Label: "", Interval: 1, Info: BlockInfo{FullText: "12:00", ShortText: "12:00", Name: "DateTime"}},
			{Label: "S:", Interval: 0, Info: BlockInfo{FullText: "static", Name: "StaticText"}},
		},
	}

	line, minInterval := bar.render()

	// The shortest non-zero interval drives the redraw.
	if minInterval != 1 {
		t.Errorf("minInterval = %d, want 1", minInterval)
	}
	if !strings.HasPrefix(line, ",[") || !strings.HasSuffix(line, "]") {
		t.Fatalf("line is not an i3bar array: %q", line)
	}

	var blocks []BlockInfo
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, ",")), &blocks); err != nil {
		t.Fatalf("rendered line is not valid JSON: %s\n%s", err, line)
	}
	if len(blocks) != 3 {
		t.Fatalf("rendered %d blocks, want 3", len(blocks))
	}
	if blocks[0].FullText != "C: 50 %" {
		t.Errorf("labelled block = %q, want %q", blocks[0].FullText, "C: 50 %")
	}
	// An empty label must not leave a leading space behind.
	if blocks[1].FullText != "12:00" {
		t.Errorf("unlabelled block = %q, want %q", blocks[1].FullText, "12:00")
	}
}

func TestRenderAllIntervalsZero(t *testing.T) {
	bar := &Bar{
		log:    xlog.NopLogger,
		blocks: []Block{{Interval: 0, Info: BlockInfo{FullText: "static"}}},
	}
	if _, minInterval := bar.render(); minInterval != 0 {
		t.Errorf("minInterval = %d, want 0 so printItems stops instead of spinning", minInterval)
	}
}

func TestStopIsIdempotent(t *testing.T) {
	// Reloading stops the previous bar; a second Stop must not panic on a
	// closed channel, and a bar that never started has no channel at all.
	(&Bar{}).Stop()

	bar := &Bar{stop: make(chan bool)}
	bar.Stop()
	bar.Stop()

	select {
	case <-bar.stop:
	default:
		t.Error("stop channel was not closed")
	}
}

// panicModule stands in for a module whose click handler blows up, which is
// exactly what an empty ticket list used to do in Toggl and Clockify.
type panicModule struct {
	BaseModule
	stopped bool
}

func (*panicModule) UpdateInfo(info BlockInfo) BlockInfo { return info }

func (*panicModule) HandleClick(cm ClickMessage, info BlockInfo) (*BlockInfo, error) {
	panic("index out of range [-1]")
}

func (p *panicModule) Stop() { p.stopped = true }

type okModule struct {
	BaseModule
}

func (*okModule) UpdateInfo(info BlockInfo) BlockInfo { return info }

func (*okModule) HandleClick(cm ClickMessage, info BlockInfo) (*BlockInfo, error) {
	info.FullText = "clicked"
	return &info, nil
}

func TestHandleClickMessageSurvivesAPanickingModule(t *testing.T) {
	// There is one click goroutine for the whole bar and main's recover only
	// covers the main goroutine, so an unrecovered panic here killed the process
	// instead of breaking a single block.
	bar := &Bar{
		log: xlog.NopLogger,
		blocks: []Block{
			{Interval: 1, Info: BlockInfo{Name: "Boom", Instance: "id_0"}, module: &panicModule{}},
			{Interval: 1, Info: BlockInfo{Name: "Fine", Instance: "id_1"}, module: &okModule{}},
		},
	}

	bar.handleClickMessage(ClickMessage{Name: "Boom", Instance: "id_0", Button: 5})

	// The bar still serves the other blocks.
	bar.handleClickMessage(ClickMessage{Name: "Fine", Instance: "id_1", Button: 1})
	if bar.blocks[1].Info.FullText != "clicked" {
		t.Errorf("second block full text = %q, want %q", bar.blocks[1].Info.FullText, "clicked")
	}
}

func TestStopStopsTheModules(t *testing.T) {
	// A config reload builds a whole new set of modules; without this the old
	// ones keep their goroutines polling forever.
	module := &panicModule{}
	bar := &Bar{
		log:    xlog.NopLogger,
		stop:   make(chan bool),
		blocks: []Block{{Info: BlockInfo{Name: "Boom"}, module: module}},
	}

	bar.Stop()

	if !module.stopped {
		t.Error("Bar.Stop() did not stop the module")
	}
	bar.Stop() // still idempotent
}

func TestCreateModuleReportsTheErrorOnce(t *testing.T) {
	block := &Block{ModuleName: "NoSuchModule"}
	err := block.CreateModule(0, xlog.NopLogger)
	if err == nil {
		t.Fatal("CreateModule() = nil, want an error for an unknown module")
	}

	bar := &Bar{log: xlog.NopLogger, blocks: []Block{*block}}
	line, _ := bar.render()

	var blocks []BlockInfo
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, ",")), &blocks); err != nil {
		t.Fatalf("rendered line is not valid JSON: %s\n%s", err, line)
	}
	// render prepends the label, so putting the message in both used to print it
	// twice: "ERR: module not found: `X` module not found: `X`".
	want := "ERR: module not found: `NoSuchModule`"
	if blocks[0].FullText != want {
		t.Errorf("full text = %q, want %q", blocks[0].FullText, want)
	}
}

func TestMapDefaultsWithoutDefaultsSection(t *testing.T) {
	// A config without a `defaults` section used to panic here, which left
	// createBar returning nil and took the process down with it.
	defaults = defaultsValueFor(nil)
	info := BlockInfo{FullText: "text"}
	mapDefaults(&info)
	if info.TextColor != "" {
		t.Errorf("color = %q, want empty", info.TextColor)
	}

	defaults = defaultsValueFor(&BlockInfo{TextColor: "#ffffff", BorderBottom: 2})
	info = BlockInfo{FullText: "text", BorderBottom: 5}
	mapDefaults(&info)
	if info.TextColor != "#ffffff" {
		t.Errorf("color = %q, want the default to fill it in", info.TextColor)
	}
	if info.BorderBottom != 5 {
		t.Errorf("border_bottom = %d, want the block's own 5 to win", info.BorderBottom)
	}
}
