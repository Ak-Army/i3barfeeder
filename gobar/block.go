package gobar

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Ak-Army/xlog"
)

type BlockInfo struct {
	FullText            string      `config:"full_text" json:"full_text"`
	ShortText           string      `config:"short_text" json:"short_text,omitempty"`
	TextColor           string      `config:"color" json:"color,omitempty"`
	BackgroundColor     string      `config:"background" json:"background,omitempty"`
	BorderColor         string      `config:"border" json:"border,omitempty"`
	MinWidth            int         `config:"min_width" json:"min_width,omitempty"`
	Align               BlockAlign  `config:"align" json:"align,omitempty"`
	Name                string      `config:"name" json:"name"`
	Instance            string      `config:"instance" json:"instance,omitempty"`
	IsUrgent            bool        `config:"urgent" json:"urgent,omitempty"`
	HasSeparator        bool        `config:"separator" json:"separator,omitempty"`
	SeparatorBlockWidth int         `config:"separator_block_width" json:"separator_block_width,omitempty"`
	Markup              BlockMarkup `config:"markup" json:"markup,omitempty"`
	BorderTop           int         `config:"border_top" json:"border_top"`
	BorderBottom        int         `config:"border_bottom" json:"border_bottom"`
	BorderLeft          int         `config:"border_left" json:"border_left"`
	BorderRight         int         `config:"border_right" json:"border_right"`
}

// Block i3  item
type Block struct {
	ModuleName string          `config:"module" json:"module"`
	Label      string          `config:"label" json:"label"`
	Interval   int64           `config:"interval" json:"interval"`
	Info       BlockInfo       `config:"info" json:"info,omitempty"`
	Config     json.RawMessage `config:"config" json:"config,omitempty"`
	module     ModuleInterface
	lastUpdate int64
}

type UpdateChannelMsg struct {
	ID   int
	Info BlockInfo
}

func (block *Block) CreateModule(id int, log xlog.Logger) error {
	block.Info.Instance = fmt.Sprintf("id_%d", id)
	if block.Info.Name == "" {
		block.Info.Name = block.ModuleName
	}
	var err error
	if module, ok := moduleRegistry[block.ModuleName]; ok {
		block.module = module()
		err = block.module.InitModule(block.Config, log)
	} else {
		err = fmt.Errorf("module not found: `%s`", block.ModuleName)
	}
	if err != nil {
		block.Label = "ERR: " + err.Error()
		block.Info = BlockInfo{
			TextColor: "#FF0000",
			FullText:  err.Error(),
			Name:      "StaticText",
		}
		block.Config = json.RawMessage{}
		block.module = moduleRegistry["StaticText"]()
		block.module.InitModule(block.Config, log)
	}
	return err
}

func (block Block) Start(ID int, updateChannel chan<- UpdateChannelMsg) {
	for {
		newInfo := block.module.UpdateInfo(block.Info)
		m := UpdateChannelMsg{
			ID:   ID,
			Info: newInfo,
		}
		updateChannel <- m
		block.lastUpdate = time.Now().Unix()
		if block.Interval == 0 {
			break
		}
		time.Sleep(time.Duration(block.Interval) * time.Second)
	}
}

func (block Block) HandleClick(cm ClickMessage) (*BlockInfo, error) {
	return block.module.HandleClick(cm, block.Info)
}
