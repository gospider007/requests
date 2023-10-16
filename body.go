package requests

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"

	"github.com/gospider007/gson"
	"github.com/gospider007/tools"
)

type File struct {
	Name        string
	FileName    string
	Content     []byte
	ContentType string
}
type bodyType = int

const (
	jsonType = iota
	textType
	rawType
	dataType
	formType
	paramsType
)

func newBody(val any, valType bodyType, dataMap map[string][]string) (*bytes.Reader, error) {
	switch value := val.(type) {
	case *gson.Client:
		if !value.IsObject() {
			return nil, errors.New("body-type error")
		}
		switch valType {
		case jsonType, textType, rawType:
			return bytes.NewReader(value.Bytes()), nil
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
			return bytes.NewReader(tools.StringToBytes(tempVal.Encode())), nil
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
			return nil, nil
		default:
			return nil, fmt.Errorf("unknow content-type：%d", valType)
		}
	case string:
		switch valType {
		case jsonType, textType, dataType, rawType:
			return bytes.NewReader(tools.StringToBytes(value)), nil
		default:
			return nil, fmt.Errorf("unknow content-type：%d", valType)
		}
	case []byte:
		switch valType {
		case jsonType, textType, dataType, rawType:
			return bytes.NewReader(value), nil
		default:
			return nil, fmt.Errorf("unknow content-type：%d", valType)
		}
	default:
		result, err := gson.Decode(value)
		if err != nil {
			return nil, err
		}
		return newBody(result, valType, dataMap)
	}
}
