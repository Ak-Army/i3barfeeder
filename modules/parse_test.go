package modules

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

// Real iwconfig output, the shape the parser has to survive.
const iwconfigWireless = `wlp2s0    IEEE 802.11  ESSID:"Hunyi-2.4"
          Mode:Managed  Frequency:2.422 GHz  Access Point: 80:3F:5D:75:EF:BE
          Bit Rate=103.2 Mb/s   Tx-Power=3 dBm
          Retry short limit:7   RTS thr:off   Fragment thr:off
          Power Management:on
          Link Quality=51/70  Signal level=-59 dBm
          Rx invalid nwid:0  Rx invalid crypt:0  Rx invalid frag:0
`

const iwconfigNotAssociated = `wlp2s0    IEEE 802.11  ESSID:off/any
          Mode:Managed  Access Point: Not-Associated   Tx-Power=3 dBm
`

func TestParseWireless(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{name: "ssid and signal level", out: iwconfigWireless, want: "Hunyi-2.4 (-59 dB)"},
		{name: "not associated", out: iwconfigNotAssociated, want: "wlp2s0"},
		{
			// Some drivers report the SSID but no signal level; that used to
			// panic with an index out of range.
			name: "ssid without a signal level",
			out:  `wlp2s0    IEEE 802.11  ESSID:"Cafe WiFi"  ` + "\n          Mode:Managed\n",
			want: "Cafe WiFi",
		},
		{name: "wired interface output", out: "eth0      no wireless extensions.\n", want: "wlp2s0"},
		{name: "empty output", out: "", want: "wlp2s0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseWireless(tc.out, "wlp2s0"); got != tc.want {
				t.Errorf("parseWireless() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Real `pactl list sinks` output under LC_ALL=C, trimmed to what the regexes need.
const pactlSink = `Sink #0
	State: RUNNING
	Name: alsa_output.pci-0000_00_1f.3.analog-stereo
	Description: Built-in Audio Analog Stereo
	Mute: no
	Volume: front-left: 52428 /  80% / -5.81 dB,   front-right: 52428 /  80% / -5.81 dB
	Base Volume: 65536 / 100% / 0.00 dB
`

const pactlSinkMuted = `Sink #1
	State: SUSPENDED
	Name: alsa_output.usb-headset
	Mute: off
	Volume: front-left: 32768 /  50% / -18.06 dB,   front-right: 32768 /  50% / -18.06 dB
`

func TestVolumeInfoParse(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want float64
	}{
		{name: "unmuted sink", out: pactlSink, want: 80},
		{name: "muted sink reports zero", out: pactlSinkMuted, want: 0},
		{name: "no sinks at all", out: "", want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &VolumeInfo{}
			if err := m.InitModule(nil, xlog.NopLogger); err != nil {
				t.Fatalf("InitModule: %s", err)
			}
			if got := m.volumeInfo(tc.out); got != tc.want {
				t.Errorf("volumeInfo() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestVolumeCmdUsesConfiguredStep(t *testing.T) {
	m := &VolumeInfo{Step: 3}
	if err := m.InitModule(nil, xlog.NopLogger); err != nil {
		t.Fatalf("InitModule: %s", err)
	}
	want := "pactl set-sink-mute @DEFAULT_SINK@ false; pactl set-sink-volume @DEFAULT_SINK@ +3%"
	if got := m.volumeCmd("+"); got != want {
		t.Errorf("volumeCmd(+) = %q, want %q", got, want)
	}
	want = "pactl set-sink-mute @DEFAULT_SINK@ false; pactl set-sink-volume @DEFAULT_SINK@ -3%"
	if got := m.volumeCmd("-"); got != want {
		t.Errorf("volumeCmd(-) = %q, want %q", got, want)
	}

	// A missing or nonsensical step falls back to the historical 5%.
	zero := &VolumeInfo{}
	if err := zero.InitModule(nil, xlog.NopLogger); err != nil {
		t.Fatalf("InitModule: %s", err)
	}
	if zero.Step != 5 {
		t.Errorf("Step = %d, want the 5%% fallback", zero.Step)
	}
}

func TestDelta(t *testing.T) {
	tests := []struct {
		name       string
		curr, prev uint64
		want       uint64
	}{
		{name: "normal growth", curr: 1500, prev: 1000, want: 500},
		{name: "no traffic", curr: 1000, prev: 1000, want: 0},
		// The interface disappearing resets collectData to zero; an unchecked
		// subtraction would render as exabytes.
		{name: "counter reset", curr: 0, prev: 9000, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := delta(tc.curr, tc.prev); got != tc.want {
				t.Errorf("delta(%d, %d) = %d, want %d", tc.curr, tc.prev, got, tc.want)
			}
		})
	}
}

func TestWrapIndex(t *testing.T) {
	tests := []struct {
		name       string
		i, step, n int
		want       int
	}{
		{name: "forward", i: 0, step: 1, n: 3, want: 1},
		{name: "backward", i: 2, step: -1, n: 3, want: 1},
		{name: "forward past the end", i: 2, step: 1, n: 3, want: 0},
		{name: "backward past the start", i: 0, step: -1, n: 3, want: 2},
		// The ticket list shrinks whenever a project stops resolving. An index
		// left over from the longer list used to be handed straight to the
		// slice, which panicked.
		{name: "stale index, scrolling up", i: 7, step: 1, n: 3, want: 0},
		{name: "stale index, scrolling down", i: 7, step: -1, n: 3, want: 2},
		{name: "single ticket", i: 0, step: 1, n: 1, want: 0},
		// len(tickets)-1 on an empty list is -1, the index that took the whole
		// process down.
		{name: "empty list, scrolling down", i: 0, step: -1, n: 0, want: 0},
		{name: "empty list, scrolling up", i: 0, step: 1, n: 0, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapIndex(tc.i, tc.step, tc.n)
			if got != tc.want {
				t.Errorf("wrapIndex(%d, %d, %d) = %d, want %d", tc.i, tc.step, tc.n, got, tc.want)
			}
			if tc.n > 0 && (got < 0 || got >= tc.n) {
				t.Errorf("wrapIndex(%d, %d, %d) = %d, out of range", tc.i, tc.step, tc.n, got)
			}
		})
	}
}

func TestHasTicketsOnEmptyList(t *testing.T) {
	// Nothing loads the ticket list until the first successful project fetch, so
	// every m.tickets index has to be guarded by this.
	toggl := &Toggl{log: xlog.NopLogger}
	if toggl.hasTickets() {
		t.Error("Toggl.hasTickets() = true on an empty list")
	}
	toggl.tickets = []ticket{{name: "ENG-1"}}
	if !toggl.hasTickets() {
		t.Error("Toggl.hasTickets() = false with one ticket")
	}

	clockify := &Clockify{log: xlog.NopLogger}
	if clockify.hasTickets() {
		t.Error("Clockify.hasTickets() = true on an empty list")
	}
	clockify.tickets = []cticket{{name: "ENG-1"}}
	if !clockify.hasTickets() {
		t.Error("Clockify.hasTickets() = false with one ticket")
	}
}

func TestReadLinesStopsOnError(t *testing.T) {
	// The loop used to run until it saw exactly io.EOF, so any other error left
	// it spinning forever while feeding the callback empty lines. A directory
	// reads as EISDIR, which is the shape that used to hang.
	dir := t.TempDir()
	done := make(chan int, 1)
	go func() {
		var calls int
		readLines(dir, func(string) bool {
			calls++
			if calls > 1000 {
				panic("readLines did not stop on a non-EOF error")
			}
			return true
		})
		done <- calls
	}()

	select {
	case calls := <-done:
		if calls != 0 {
			t.Errorf("callback ran %d times on an unreadable file, want 0", calls)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("readLines did not return")
	}
}

func TestReadLinesReadsEveryLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proc")
	if err := os.WriteFile(path, []byte("first\nsecond\nthird\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %s", err)
	}
	var got []string
	readLines(path, func(line string) bool {
		got = append(got, line)
		return true
	})
	want := []string{"first", "second", "third"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readLines() = %q, want %q", got, want)
	}

	// A callback returning false stops the walk.
	var stopped []string
	readLines(path, func(line string) bool {
		stopped = append(stopped, line)
		return false
	})
	if len(stopped) != 1 {
		t.Errorf("callback ran %d times after returning false, want 1", len(stopped))
	}
}

func TestNetworkKeepsBaselineWhenInterfaceIsMissing(t *testing.T) {
	m := &Network{InterfaceName: []string{"definitely-not-an-interface"}, log: xlog.NopLogger}
	m.currRx, m.currTx = 1000, 2000

	m.UpdateInfo(gobar.BlockInfo{})

	if m.currRx != 1000 || m.currTx != 2000 {
		t.Errorf("baseline = (%d, %d), want it left at (1000, 2000)", m.currRx, m.currTx)
	}
	m.collectData()
	if m.currRx != 0 || m.currTx != 0 {
		t.Error("collectData() reported ok for a missing interface")
	}
}

func TestByteSize(t *testing.T) {
	tests := []struct {
		in   uint64
		want string
	}{
		{in: 0, want: "0 B"},
		{in: 999, want: "999 B"},
		{in: 1024, want: "1.0 kB"},
		{in: 1536, want: "1.5 kB"},
		{in: 1048576, want: "1.0 MB"},
		{in: 1073741824, want: "1.0 GB"},
	}
	for _, tc := range tests {
		if got := byteSize(tc.in); got != tc.want {
			t.Errorf("byteSize(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMakeBar(t *testing.T) {
	cfg := barConfig{BarSize: 10, BarFull: "#", BarEmpty: "."}
	tests := []struct {
		percent float64
		want    string
	}{
		{percent: 0, want: ".........."},
		{percent: 50, want: "#####....."},
		{percent: 100, want: "##########"},
		{percent: 105, want: "##########"}, // must not overflow the bar
	}
	for _, tc := range tests {
		got := makeBar(tc.percent, cfg)
		if got != tc.want {
			t.Errorf("makeBar(%v) = %q, want %q", tc.percent, got, tc.want)
		}
		if len([]rune(got)) != cfg.BarSize {
			t.Errorf("makeBar(%v) has %d cells, want %d", tc.percent, len([]rune(got)), cfg.BarSize)
		}
	}
}
