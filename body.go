package requests

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/gospider007/gson"
	"github.com/gospider007/tools"
)

type bodyType = int

const (
	jsonType = iota
	textType
	rawType
	dataType
	formType
	paramsType
)

func (obj *RequestOption) newBody(val any, valType bodyType, dataMap map[string][]string) error {
	if reader, ok := val.(io.Reader); ok {
		obj.once = true
		obj.body = reader
		return nil
	}
	switch value := val.(type) {
	case *gson.Client:
		if !value.IsObject() {
			return errors.New("body-type error")
		}
		switch valType {
		case jsonType, textType, rawType:
			obj.body = bytes.NewReader(value.Bytes())
			return nil
		case dataType:
			tempVal := url.Values{}
			for kk, vv := range value.Map() {
				if vv.IsArray() {
					for _, v := range vv.Array() {
						tempVal.Add(kk, v.String())
					}
				} else {
					tempVal.Add(kk, vv.String())
				}
			}
			obj.body = bytes.NewReader(tools.StringToBytes(tempVal.Encode()))
			return nil
		case formType, paramsType:
			for kk, vv := range value.Map() {
				kkvv := []string{}
				if vv.IsArray() {
					for _, v := range vv.Array() {
						kkvv = append(kkvv, v.String())
					}
				} else {
					kkvv = append(kkvv, vv.String())
				}
				dataMap[kk] = kkvv
			}
			return nil
		default:
			return fmt.Errorf("unknow content-type：%d", valType)
		}
	case string:
		switch valType {
		case jsonType, textType, dataType, rawType:
			obj.body = bytes.NewReader(tools.StringToBytes(value))
			return nil
		case formType, paramsType:
		default:
			return fmt.Errorf("unknow content-type：%d", valType)
		}
	case []byte:
		switch valType {
		case jsonType, textType, dataType, rawType:
			obj.body = bytes.NewReader(value)
			return nil
		case formType, paramsType:
		default:
			return fmt.Errorf("unknow content-type：%d", valType)
		}
	}
	result, err := gson.Decode(val)
	if err != nil {
		return err
	}
	return obj.newBody(result, valType, dataMap)
}
