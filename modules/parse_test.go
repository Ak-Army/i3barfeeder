package modules

import (
	"testing"

	"github.com/Ak-Army/xlog"
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
