package gobar

import (
	"encoding/json"
	"reflect"

	"github.com/Ak-Army/xlog"
)

const (
	AlignCenter BlockAlign = "center"
	AlignRight  BlockAlign = "right"
	AlignLeft   BlockAlign = "left"

	MarkupNone  BlockMarkup = "none"
	MarkupPango BlockMarkup = "pango"
)

type ModuleInterface interface {
	InitModule(config json.RawMessage, log xlog.Logger) error
	UpdateInfo(info BlockInfo) BlockInfo
	HandleClick(cm ClickMessage, info BlockInfo) (*BlockInfo, error)
}

type BlockMarkup string

type BlockAlign string

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
		Str:   align,
	}
}

func (ba *BlockAlign) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(*ba))
}

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
		Str:   markup,
	}
}

func (bm *BlockMarkup) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(*bm))
}
