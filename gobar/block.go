package gobar

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"time"
)

type BlockAlign string

const (
	AlignCenter BlockAlign = "center"
	AlignRight  BlockAlign = "right"
	AlignLeft   BlockAlign = "left"
)

func (ba *BlockAlign) UnmarshalJSON(data []byte) error {
	var align string
	if err := json.Unmarshal(data, &align); err != nil {
		return err
	}
	switch align {
	case string(AlignCenter):
		*ba = AlignCenter
		return nil
	case string(AlignRight):
		*ba = AlignRight
		return nil
	case string(AlignLeft):
		*ba = AlignLeft
		return nil
	}
	return &json.UnsupportedValueError{
		Value: reflect.ValueOf(align),
		Str:   string(align),
	}
}
func (ba *BlockAlign) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(*ba))
}

type BlockMarkup string

const (
	MarkupNone  BlockMarkup = "none"
	MarkupPango BlockMarkup = "pango"
)

func (bm *BlockMarkup) UnmarshalJSON(data []byte) error {
	var markup string
	if err := json.Unmarshal(data, &markup); err != nil {
		return err
	}
	switch markup {
	case string(MarkupNone):
		*bm = MarkupNone
		return nil
	case string(MarkupPango):
		*bm = MarkupPango
		return nil
	}

	return &json.UnsupportedValueError{
		Value: reflect.ValueOf(markup),
		Str:   string(markup),
	}
}
func (bm *BlockMarkup) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(*bm))
}

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

type Config map[string]interface{}

type ModuleInterface interface {
	InitModule(config Config) error
	UpdateInfo(info BlockInfo) BlockInfo
	HandleClick(cm ClickMessage, info BlockInfo) (*BlockInfo, error)
}

// Block i3  item
type Block struct {
	ModuleName string    `json:"module"`
	Label      string    `json:"label"`
	Interval   int64     `json:"interval"`
	Info       BlockInfo `json:"info,omitempty"`
	Config     Config    `json:"config,omitempty"`
	module     ModuleInterface
	lastUpdate int64
}

type UpdateChannelMsg struct {
	ID   int
	Info BlockInfo
}

func (block *Block) CreateModule(id int, logger *log.Logger) (err error) {
	var ok bool
	if name, ok := typeRegistry[block.ModuleName]; ok {
		v := reflect.New(name)
		if block.module, ok = v.Interface().(ModuleInterface); ok {
			err = block.module.InitModule(block.Config)
		} else {
			err = fmt.Errorf("Cannot create instance of `%s`", name)
		}
	} else {
		err = fmt.Errorf("Module not found: `%s`", block.ModuleName)
	}
	block.Info.Instance = fmt.Sprintf("id_%d", id)
	if block.Info.Name == "" {
		block.Info.Name = block.ModuleName
	}

	if err != nil {
		block.Label = "ERR: "
		block.Info = BlockInfo{
			TextColor: "#FF0000",
			FullText:  err.Error(),
			Name:      "StaticText",
		}
		block.Config = Config{}
		v := reflect.New(typeRegistry["StaticText"])
		if block.module, ok = v.Interface().(ModuleInterface); ok {
			block.module.InitModule(block.Config)
		}
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

var typeRegistry = make(map[string]reflect.Type)

func AddModule(name string, module reflect.Type) {
	typeRegistry[name] = module
}
