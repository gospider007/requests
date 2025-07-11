package requests

import (
	"bytes"
	"encoding/hex"
	"errors"
	"net/textproto"
	"slices"
	"strings"

	"github.com/gospider007/http2"
	"github.com/gospider007/ja3"
	"github.com/gospider007/tools"
)

func (obj *Spec) Map() map[string]any {
	results := map[string]any{
		"orderHeaders": obj.OrderHeaders,
		"raw":          obj.String(),
	}
	return results
}
func (obj *Spec) Hex() string {
	return tools.Hex(obj.raw)
}
func (obj *Spec) Bytes() []byte {
	return obj.raw
}
func (obj *Spec) String() string {
	return tools.BytesToString(obj.raw)
}

type Spec struct {
	OrderHeaders [][2]string
	raw          []byte
}

func ParseSpec(raw []byte) (*Spec, error) {
	i := bytes.Index(raw, []byte("\r\n\r\n"))
	if i == -1 {
		return nil, errors.New("not found \\r\\n")
	}
	rawContent := raw[:i]
	orderHeaders := [][2]string{}
	for i, line := range bytes.Split(rawContent, []byte("\r\n")) {
		if i == 0 {
			continue
		}
		ols := bytes.Split(line, []byte(": "))
		if len(ols) < 2 {
			return nil, errors.New("not found header")
		}
		orderHeaders = append(orderHeaders, [2]string{
			tools.BytesToString(ols[0]),
			tools.BytesToString(bytes.Join(ols[1:], []byte(": "))),
		})
	}
	return &Spec{
		raw:          raw,
		OrderHeaders: orderHeaders,
	}, nil
}

type GospiderSpec struct {
	TLSSpec *ja3.Spec
	H1Spec  *Spec
	H2Spec  *http2.Spec
}

func ParseGospiderSpec(value string) (*GospiderSpec, error) {
	specs := strings.Split(value, "@")
	spec := new(GospiderSpec)
	if len(specs) != 3 {
		return nil, errors.New("spec format error")
	}
	if specs[0] != "" {
		b, err := hex.DecodeString(specs[0])
		if err != nil {
			return nil, err
		}
		if spec.TLSSpec, err = ja3.ParseSpec(b); err != nil {
			return nil, err
		}
	}
	if specs[1] != "" {
		b, err := hex.DecodeString(specs[1])
		if err != nil {
			return nil, err
		}
		if spec.H1Spec, err = ParseSpec(b); err != nil {
			return nil, err
		}
	}
	if specs[2] != "" {
		b, err := hex.DecodeString(specs[2])
		if err != nil {
			return nil, err
		}
		if spec.H2Spec, err = http2.ParseSpec(b); err != nil {
			return nil, err
		}
	}
	return spec, nil
}

func (obj *RequestOption) initSpec() error {
	if obj.Spec == "" {
		return nil
	}
	gospiderSpec, err := ParseGospiderSpec(obj.Spec)
	if err != nil {
		return err
	}
	obj.gospiderSpec = gospiderSpec
	if obj.orderHeaders == nil {
		if gospiderSpec.H1Spec != nil {
			obj.orderHeaders = NewOrderData()
			for _, kv := range gospiderSpec.H1Spec.OrderHeaders {
				if slices.Contains(tools.DefaultHeaderKeys, kv[0]) {
					obj.orderHeaders.Add(kv[0], kv[1])
				} else {
					obj.orderHeaders.Add(kv[0], nil)
				}
			}
		} else if gospiderSpec.H2Spec != nil {
			obj.orderHeaders = NewOrderData()
			for _, kv := range gospiderSpec.H2Spec.OrderHeaders {
				key := textproto.CanonicalMIMEHeaderKey(kv[0])
				if slices.Contains(tools.DefaultHeaderKeys, key) {
					obj.orderHeaders.Add(key, kv[1])
				} else {
					obj.orderHeaders.Add(key, nil)
				}
			}
		}
		if len(obj.OrderHeaders) > 0 && obj.orderHeaders != nil {
			obj.orderHeaders.ReorderWithKeys(obj.OrderHeaders...)
		}
	}
	return nil
}
