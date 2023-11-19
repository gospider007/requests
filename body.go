package requests

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"reflect"

	"github.com/gospider007/gson"
	"github.com/gospider007/tools"
)

const (
	readType = iota
	mapType
)

type orderMap struct {
	data  map[string]any
	order []string
}

func NewOrderMap() *orderMap {
	return &orderMap{
		data:  make(map[string]any),
		order: []string{},
	}
}

func (obj *orderMap) Set(key string, val any) {
	obj.Del(key)
	obj.data[key] = val
	obj.order = append(obj.order, key)
}
func (obj *orderMap) Del(key string) {
	delete(obj.data, key)
	obj.order = tools.DelSliceVals(obj.order, key)
}
func (obj *orderMap) parseHeaders() (map[string][]string, []string) {
	if len(obj.order) == 0 || len(obj.data) == 0 {
		return nil, nil
	}
	head := make(http.Header)
	for kk, vv := range obj.data {
		if vvs, ok := vv.([]string); ok {
			for _, vv := range vvs {
				head.Add(kk, fmt.Sprint(vv))
			}
		} else {
			head.Add(kk, fmt.Sprint(vv))
		}
	}
	return head, obj.order
}

func formWrite(writer *multipart.Writer, key string, val any) (err error) {
	switch value := val.(type) {
	case File:
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(key), escapeQuotes(value.FileName)))
		if value.ContentType == "" {
			h.Set("Content-Type", http.DetectContentType(value.Content))
		} else {
			h.Set("Content-Type", value.ContentType)
		}
		var wp io.Writer
		if wp, err = writer.CreatePart(h); err != nil {
			return
		}
		_, err = wp.Write(value.Content)
	default:
		err = writer.WriteField(key, fmt.Sprint(value))
	}
	return
}
func (obj *orderMap) parseForm() (tempBody *bytes.Buffer, contentType string, err error) {
	if len(obj.order) == 0 || len(obj.data) == 0 {
		return
	}
	tempBody = bytes.NewBuffer(nil)
	writer := multipart.NewWriter(tempBody)
	for _, key := range obj.order {
		if vals, ok := obj.data[key].([]any); ok {
			for _, val := range vals {
				formWrite(writer, key, val)
			}
		} else {
			formWrite(writer, key, obj.data[key])
		}
	}
	if err = writer.Close(); err != nil {
		return
	}
	contentType = writer.FormDataContentType()
	return
}
func paramsWrite(buf *bytes.Buffer, key string, val any) {
	if buf.Len() > 0 {
		buf.WriteByte('&')
	}
	buf.WriteString(url.QueryEscape(key))
	buf.WriteByte('=')
	buf.WriteString(url.QueryEscape(fmt.Sprint(val)))
}
func (obj *orderMap) parseParams() string {
	if len(obj.order) == 0 || len(obj.data) == 0 {
		return ""
	}
	buf := bytes.NewBuffer(nil)
	for _, k := range obj.order {
		if vals, ok := obj.data[k].([]any); ok {
			for _, v := range vals {
				paramsWrite(buf, k, v)
			}
		} else {
			paramsWrite(buf, k, obj.data[k])
		}
	}
	return buf.String()
}
func (obj *orderMap) parseData() *bytes.Reader {
	if len(obj.order) == 0 || len(obj.data) == 0 {
		return nil
	}
	tempVal := url.Values{}
	for kk, vv := range obj.data {
		if vvs, ok := vv.([]any); ok {
			for _, vv := range vvs {
				tempVal.Add(kk, fmt.Sprint(vv))
			}
		} else {
			tempVal.Add(kk, fmt.Sprint(vv))
		}
	}
	return bytes.NewReader(tools.StringToBytes(tempVal.Encode()))
}

func (obj *orderMap) MarshalJSON() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := buf.WriteByte('{')
	if err != nil {
		return nil, err
	}
	for i, k := range obj.order {
		if i > 0 {
			if err = buf.WriteByte(','); err != nil {
				return nil, err
			}
		}
		key, err := gson.Encode(k)
		if err != nil {
			return nil, err
		}
		if _, err = buf.Write(key); err != nil {
			return nil, err
		}
		if err = buf.WriteByte(':'); err != nil {
			return nil, err
		}
		val, err := gson.Encode(obj.data[k])
		if err != nil {
			return nil, err
		}
		if _, err = buf.Write(val); err != nil {
			return nil, err
		}
	}
	if err = buf.WriteByte('}'); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func any2Map(val any) map[any]any {
	mapType := reflect.TypeOf(val)
	if mapType.Kind() != reflect.Map {
		return nil
	}
	mapValue := reflect.ValueOf(val)
	keys := mapValue.MapKeys()
	result := make(map[any]any)
	for _, key := range keys {
		keyData := key.Interface()
		valueData := mapValue.MapIndex(key).Interface()
		sliceValue := reflect.ValueOf(valueData)
		if sliceValue.Kind() == reflect.Slice {
			valueData2 := []any{}
			for i := 0; i < sliceValue.Len(); i++ {
				valueData2 = append(valueData2, sliceValue.Index(i).Interface())
			}
			valueData = valueData2
		}
		result[keyData] = valueData
	}
	return result
}

func (obj *RequestOption) newBody(val any, valType int) (io.Reader, *orderMap, []string, error) {
	if reader, ok := val.(io.Reader); ok {
		obj.once = true
		return reader, nil, nil, nil
	}
	if mapData := any2Map(val); mapData != nil {
		val = mapData
	}
	switch value := val.(type) {
	case *gson.Client:
		if !value.IsObject() {
			return nil, nil, nil, errors.New("body-type error")
		}
		switch valType {
		case readType:
			return bytes.NewReader(value.Bytes()), nil, nil, nil
		case mapType:
			orderMap := NewOrderMap()
			for kk, vv := range value.Map() {
				if vv.IsArray() {
					valData := make([]any, len(vv.Array()))
					for i, v := range vv.Array() {
						valData[i] = v.Value()
					}
					orderMap.Set(kk, valData)
				} else {
					orderMap.Set(kk, vv.Value())
				}
			}
			return nil, orderMap, nil, nil
		default:
			return nil, nil, nil, fmt.Errorf("unknow content-type：%d", valType)
		}
	case *orderMap:
		switch valType {
		case readType:
			enData, err := gson.Encode(value)
			return bytes.NewReader(enData), nil, nil, err
		case mapType:
			return nil, value, nil, nil
		default:
			return nil, nil, nil, fmt.Errorf("unknow content-type：%d", valType)
		}
	case map[any]any:
		switch valType {
		case mapType:
			orderMap := NewOrderMap()
			for kk, vv := range value {
				if vvs, ok := vv.([]any); ok {
					vvData := make([]any, len(vvs))
					for i, vv := range vvs {
						vvData[i] = vv
					}
					orderMap.Set(fmt.Sprint(kk), vvData)
				} else {
					orderMap.Set(fmt.Sprint(kk), vv)
				}
			}
			return nil, orderMap, nil, nil
		}
	case string:
		switch valType {
		case readType:
			return bytes.NewReader(tools.StringToBytes(value)), nil, nil, nil
		case mapType:
		default:
			return nil, nil, nil, fmt.Errorf("unknow content-type：%d", valType)
		}
	case []byte:
		switch valType {
		case readType:
			return bytes.NewReader(value), nil, nil, nil
		case mapType:
		default:
			return nil, nil, nil, fmt.Errorf("unknow content-type：%d", valType)
		}
	}
	result, err := gson.Decode(val)
	if err != nil {
		return nil, nil, nil, err
	}
	return obj.newBody(result, valType)
}
