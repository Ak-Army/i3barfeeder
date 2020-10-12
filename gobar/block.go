package gobar

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Ak-Army/xlog"
)

type BlockInfo struct {
	FullText            string      `json:"full_text"`
	ShortText           string      `json:"short_text,omitempty"`
	TextColor           string      `json:"color,omitempty"`
	BackgroundColor     string      `json:"background,omitempty"`
	BorderColor         string      `json:"border,omitempty"`
	MinWidth            int         `json:"min_width,omitempty"`
	Align               BlockAlign  `json:"align,omitempty"`
	Name                string      `json:"name"`
	Instance            string      `json:"instance,omitempty"`
	IsUrgent            bool        `json:"urgent,omitempty"`
	HasSeparator        bool        `json:"separator,omitempty"`
	SeparatorBlockWidth int         `json:"separator_block_width,omitempty"`
	Markup              BlockMarkup `json:"markup,omitempty"`
	BorderTop           int         `json:"border_top"`
	BorderBottom        int         `json:"border_bottom"`
	BorderLeft          int         `json:"border_left"`
	BorderRight         int         `json:"border_right"`
}

// Block i3  item
type Block struct {
	ModuleName string       `json:"module"`
	Label      string       `json:"label"`
	Interval   int64        `json:"interval"`
	Info       BlockInfo    `json:"info,omitempty"`
	Config     json.RawMessage `json:"config,omitempty"`
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
		block.Label = "ERR: "
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
