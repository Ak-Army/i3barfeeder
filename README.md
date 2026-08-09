# i3barfeeder

Feed i3bar with go.

A status line generator for [i3](https://i3wm.org/): it speaks the i3bar protocol
on stdout, reads click events from stdin, and builds the bar from a JSON config
file. Each entry on the bar is a *block* backed by a *module* — a small Go type
that produces the text and reacts to clicks.

## Build

```sh
go build -o i3barfeeder .
```

## Usage

```
i3barfeeder -c <config> [-k <keyring>] [-l <logfile>]
```

| Flag | Default | Description |
|---|---|---|
| `-c`, `-config` | — | Path to the JSON config file. |
| `-k`, `-key` | — | Path to the key-ring file used to decrypt `ENC(...)` values. |
| `-l`, `-log` | `/dev/null` | Log file. Nothing is written to stderr, so this is where errors show up. |

Wire it into `~/.config/i3/config`:

```
bar {
    status_command /path/to/i3barfeeder -c /path/to/config.json -k /path/to/config.keyring -l /tmp/i3barfeeder.log
}
```

Use absolute paths: i3 starts `status_command` from its own working directory,
not from the directory the binary lives in.

The process exits on `SIGINT` and `SIGTERM`.

## Configuration

```json
{
  "defaults": {
    "color": "#ffffff"
  },
  "blocks": [
    {
      "module": "DateTime",
      "label": "",
      "interval": 1,
      "info": {
        "border_bottom": 2,
        "border": "#ffffff"
      },
      "config": {
        "format": "2006-01-02 15:04:05"
      }
    }
  ]
}
```

`defaults` is optional and holds a partial `info` object; every field left empty
in a block's own `info` is filled in from it.

Each entry of `blocks` takes:

| Key | Description |
|---|---|
| `module` | Registered module name, see the table below. Required. |
| `label` | Prefix printed before the module's text. |
| `interval` | Seconds between updates. `0` means the block is rendered once and never refreshed. |
| `info` | i3bar block properties (see below). |
| `config` | Module specific settings, documented per module below. |

The bar redraws on the shortest `interval` among its blocks.

### `info` properties

These map directly onto the [i3bar protocol](https://i3wm.org/docs/i3bar-protocol.html)
block fields: `full_text`, `short_text`, `color`, `background`, `border`,
`min_width`, `align` (`left`/`center`/`right`), `name`, `instance`, `urgent`,
`separator`, `separator_block_width`, `markup` (`none`/`pango`), `border_top`,
`border_bottom`, `border_left`, `border_right`.

### Encrypted values

Settings marked as encrypted (currently the Clockify and Toggl API tokens) may be
stored as `ENC(<kid>:<ciphertext>)` and are decrypted with the key-ring passed via
`-k`. The key-ring is one `<kid>: <base64 key>` entry per line, the first one being
the active key. Plain values are also accepted in those fields.

If the key-ring is missing or unreadable the bar still starts — a warning goes to
the log, and only the blocks with encrypted values fail.

### Reloading

The config file is polled once a minute. On a change the bar is rebuilt in place,
so blocks can be added, removed or reconfigured without restarting i3.

## Modules

| Module | Shows | Config keys |
|---|---|---|
| `DateTime` | Current date and time | `format`, `shortFormat`, `location` (IANA time zone) |
| `CpuInfo` | CPU utilization as a bar | bar keys |
| `MemInfo` | Used memory as a bar | bar keys |
| `DiskUsage` | Used space of a filesystem | `path` (default `/`), bar keys |
| `Battery` | Charge level as a bar | `interfaceName` (default `BAT0`), bar keys |
| `Network` | Interface throughput; SSID and signal level on wireless links | `interfaceName` (list of interface names), bar keys |
| `VolumeInfo` | PulseAudio volume as a bar | `step` (percent per scroll, default `5`), bar keys |
| `ExternalCmd` | Output of a shell command | `exec`, `exec_if`, `click_left`, `click_middle`, `click_right`, `scroll_up`, `scroll_down` |
| `StaticText` | The `info.full_text` as given | — |
| `GCal` | Next Google Calendar event | `secretFile`, `tokenFile`, `email`, `meetingLink` |
| `Clockify` | Running Clockify entry and today's total | `apiToken` (encrypted), `ticketNames` |
| `Toggl` | Running Toggl entry and today's total | `apiToken` (encrypted), `defaultWID`, `ticketNames` |

Modules that render a bar (`CpuInfo`, `MemInfo`, `DiskUsage`, `Battery`,
`Network`, `VolumeInfo`) share three keys: `barSize` (default `10`), `barFull`
(default `■`) and `barEmpty` (default `□`).

### Mouse actions

| Module | Left | Middle | Right | Scroll up / down |
|---|---|---|---|---|
| `DateTime` | opens `gsimplecal` | | | |
| `CpuInfo` | opens `gnome-system-monitor -p` | | | |
| `MemInfo` | opens `gnome-system-monitor -r` | | | |
| `DiskUsage` | opens `gnome-system-monitor -f` | | | |
| `VolumeInfo` | | | mute / unmute | volume up / down by `step` |
| `ExternalCmd` | runs `click_left` | runs `click_middle` | runs `click_right` | runs `scroll_up` / `scroll_down` |
| `GCal` | double click reloads events | jumps back to the current event | joins the meeting | steps through upcoming events |
| `Clockify`, `Toggl` | | copies the last month's daily totals to the clipboard (`xclip`) | starts / stops the timer | cycles through the configured ticket names |

### Module details

**`ExternalCmd`** runs its commands through `/bin/sh -c`. `exec` produces the
block text; `exec_if`, when set, must exit successfully first, otherwise the
block is left untouched. A failing command puts its error on the bar.

**`GCal`** authenticates with OAuth on first use: `secretFile` is the Google API
client secret, and the token is cached in `tokenFile`. `email` is your address in
the attendee list, used to tell accepted events from declined ones. `meetingLink`
maps a provider to the way its link is found in an event:

```json
"meetingLink": {
  "zoom.us": { "regex": "https://[^ ]*zoom.us/j/[^ ]*" },
  "Google Meet": { "simple": "meet.google.com" }
}
```

An event with a link is opened automatically when it starts.

**`Clockify` / `Toggl`** need `ticketNames` — the entries you cycle through with
the scroll wheel — where `name` is the description to log, `project` is the
project to match by name, and `tpId` is an optional external id:

```json
"ticketNames": [
  { "name": "ENG-301 Maintenance", "project": "Maintenance", "tpId": "" }
]
```

## Writing a module

A module is a type implementing `gobar.ModuleInterface`:

```go
type ModuleInterface interface {
	InitModule(config *config.SubConfig, log xlog.Logger) error
	UpdateInfo(info BlockInfo) BlockInfo
	HandleClick(cm ClickMessage, info BlockInfo) (*BlockInfo, error)
}
```

Embed `gobar.BaseModule` to get `InitModule` and `HandleClick` for free, leaving
`UpdateInfo` as the only method you must write. Register the module from an
`init()` function in the `modules` package — the blank import of that package in
`main.go` is what pulls every module in:

```go
type MyModule struct {
	gobar.BaseModule
	Threshold int `config:"threshold"`
}

func init() {
	gobar.AddModule("MyModule", func() gobar.ModuleInterface {
		return &MyModule{Threshold: 90} // defaults, overwritten by the config
	})
}

func (m *MyModule) InitModule(c *config.SubConfig, log xlog.Logger) error {
	if c != nil {
		if err := c.Load(m); err != nil {
			return err
		}
	}
	return nil
}
```

Embed `BaseModule`, not `ModuleInterface`: embedding the interface makes the
compiler accept a module that is missing a method, and it panics at runtime
instead.

Settings are read from the block's `config` object through `SubConfig.Load`, which
binds by **`config:"..."` struct tags** — a `json:"..."` tag is ignored and the
field silently stays empty. Use `config:"name,encrypted"` for a secret that may
arrive as `ENC(...)`.

`UpdateInfo` is called every `interval` seconds and should return quickly: it runs
on the render path, so do network calls on your own goroutine and let `UpdateInfo`
read a cached value. An error returned from `InitModule` replaces the block with a
red error message, leaving the rest of the bar working.

`HandleClick` receives the button (1 left, 2 middle, 3 right, 4 scroll up,
5 scroll down). Return the updated `*BlockInfo` to redraw immediately, or `nil` to
leave the block as it is.

## Tests

```sh
go test ./...
```

Two of them are worth knowing about, because they guard the mistake that is
hardest to notice by hand: `TestConfigFilesOnlyUseKnownKeys` checks every config
file in the repository against the keys the modules actually accept, and
`TestModulesDeclareConfigTags` fails on an exported module field that carries no
`config:"..."` tag (or still carries a `json:"..."` one). Both catch settings that
would otherwise be dropped without a word.

## Troubleshooting

Nothing is printed to stderr, so start with the log:

```sh
i3barfeeder -c config.json -k config.keyring -l /tmp/i3barfeeder.log
```

A block showing red text carries the module's own error message.

A block that stays empty, or that keeps showing its built-in default instead of
what you configured, usually means its `config` never reached the module: keys are
matched exactly against the `config:"..."` struct tags, and an unknown one is
ignored without a word. Check the spelling and the case of the key — a `json:"..."`
tag or an `InterfaceName` where the code says `interfaceName` both end up as
silently empty fields.
