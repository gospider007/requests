package requests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"

	"github.com/gospider007/gson"
	"github.com/gospider007/tools"
)

type OrderData struct {
	data []struct {
		key string
		val any
	}
}

func NewOrderData() *OrderData {
	return &OrderData{
		data: []struct {
			key string
			val any
		}{},
	}
}

func (obj *OrderData) Add(key string, val any) {
	obj.data = append(obj.data, struct {
		key string
		val any
	}{key: key, val: val})
}
func (obj *OrderData) Keys() []string {
	keys := make([]string, len(obj.data))
	for i, value := range obj.data {
		keys[i] = value.key
	}
	return keys
}

type orderT struct {
	key string
	val any
}

func (obj orderT) Key() string {
	return obj.key
}
func (obj orderT) Val() any {
	return obj.val
}

func (obj *OrderData) Data() []interface {
	Key() string
	Val() any
} {
	if obj == nil {
		return nil
	}
	keys := make([]interface {
		Key() string
		Val() any
	}, len(obj.data))
	for i, value := range obj.data {
		keys[i] = orderT{
			key: value.key,
			val: value.val,
		}
	}
	return keys
}

func formWrite(writer *multipart.Writer, key string, val any) (err error) {
	switch value := val.(type) {
	case File:
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(key), escapeQuotes(value.FileName)))
		if value.ContentType == "" {
			switch content := value.Content.(type) {
			case []byte:
				h.Set("Content-Type", http.DetectContentType(content))
			case string:
				h.Set("Content-Type", http.DetectContentType(tools.StringToBytes(content)))
			case io.Reader:
				h.Set("Content-Type", "application/octet-stream")
			default:
				con, err := gson.Encode(content)
				if err != nil {
					return err
				}
				h.Set("Content-Type", http.DetectContentType(con))
			}
		} else {
			h.Set("Content-Type", value.ContentType)
		}
		var wp io.Writer
		if wp, err = writer.CreatePart(h); err != nil {
			return
		}
		switch content := value.Content.(type) {
		case []byte:
			_, err = wp.Write(content)
		case string:
			_, err = wp.Write(tools.StringToBytes(content))
		case io.Reader:
			_, err = io.Copy(wp, content)
		default:
			con, err := gson.Encode(content)
			if err != nil {
				return err
			}
			_, err = wp.Write(con)
			if err != nil {
				return err
			}
		}
	case []byte:
		err = writer.WriteField(key, tools.BytesToString(value))
	case string:
		err = writer.WriteField(key, value)
	default:
		con, err := gson.Decode(val)
		if err != nil {
			return err
		}
		err = writer.WriteField(key, con.Raw())
		if err != nil {
			return err
		}
	}
	return
}
func (obj *OrderData) isformPip() bool {
	if len(obj.data) == 0 {
		return false
	}
	for _, value := range obj.data {
		if file, ok := value.val.(File); ok {
			if _, ok := file.Content.(io.Reader); ok {
				return true
			}
		}
	}
	return false
}
func (obj *OrderData) formWriteMain(writer *multipart.Writer) (err error) {
	for _, value := range obj.data {
		if err = formWrite(writer, value.key, value.val); err != nil {
			return
		}
	}
	return writer.Close()
}

func paramsWrite(buf *bytes.Buffer, key string, val any) {
	if buf.Len() > 0 {
		buf.WriteByte('&')
	}
	buf.WriteString(url.QueryEscape(key))
	buf.WriteByte('=')
	if v, err := gson.Decode(val); err == nil {
		buf.WriteString(url.QueryEscape(v.Raw()))
	} else {
		buf.WriteString(url.QueryEscape(fmt.Sprintf("%v", val)))
	}
}
func (obj *OrderData) MarshalJSON() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := buf.WriteByte('{')
	if err != nil {
		return nil, err
	}
	for i, value := range obj.data {
		if i > 0 {
			if err = buf.WriteByte(','); err != nil {
				return nil, err
			}
		}
		key, err := gson.Encode(value.key)
		if err != nil {
			return nil, err
		}
		if _, err = buf.Write(key); err != nil {
			return nil, err
		}
		if err = buf.WriteByte(':'); err != nil {
			return nil, err
		}
		val, err := gson.Encode(value.val)
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
func (obj *RequestOption) newBody(val any) (io.Reader, *OrderData, bool, error) {
	switch value := val.(type) {
	case *OrderData:
		return nil, value, true, nil
	case io.ReadCloser:
		obj.once = true
		return value, nil, true, nil
	case io.Reader:
		obj.once = true
		return value, nil, true, nil
	case string:
		return bytes.NewReader(tools.StringToBytes(value)), nil, true, nil
	case []byte:
		return bytes.NewReader(value), nil, true, nil
	case map[string]any:
		orderMap := NewOrderData()
		for key, val := range value {
			orderMap.Add(key, val)
		}
		return nil, orderMap, true, nil
	default:
		jsonData, err := gson.Decode(val)
		if err != nil {
			return nil, nil, false, errors.New("invalid body type")
		}
		orderMap := NewOrderData()
		for kk, vv := range jsonData.Map() {
			orderMap.Add(kk, vv.Value())
		}
		return nil, orderMap, false, nil
	}
}

func (obj *OrderData) parseParams() *bytes.Buffer {
	buf := bytes.NewBuffer(nil)
	for _, value := range obj.data {
		paramsWrite(buf, value.key, value.val)
	}
	return buf
}
func (obj *OrderData) parseForm(ctx context.Context) (io.Reader, bool, error) {
	if len(obj.data) == 0 {
		return nil, false, nil
	}
	if obj.isformPip() {
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)
		go func() {
			stop := context.AfterFunc(ctx, func() {
				pw.CloseWithError(ctx.Err())
			})
			defer stop()
			pw.CloseWithError(obj.formWriteMain(writer))
		}()
		return pr, true, nil
	}
	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	err := obj.formWriteMain(writer)
	if err != nil {
		return nil, false, err
	}
	return bytes.NewReader(body.Bytes()), false, err
}
func (obj *OrderData) parseData() io.Reader {
	val := obj.parseParams().Bytes()
	if val == nil {
		return nil
	}
	return bytes.NewReader(val)
}
func (obj *OrderData) parseJson() (io.Reader, error) {
	con, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(con), nil
}
func (obj *OrderData) parseText() (io.Reader, error) {
	con, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(con), nil
}
