package requests

import (
	"errors"
	_ "unsafe"

	"net/http"

	"github.com/gospider007/gson"
)

//go:linkname readCookies net/http.readCookies
func readCookies(h http.Header, filter string) []*http.Cookie

//go:linkname readSetCookies net/http.readSetCookies
func readSetCookies(h http.Header) []*http.Cookie

// 支持json,map,[]string,http.Header,string
func ReadCookies(val any) (Cookies, error) {
	switch cook := val.(type) {
	case *http.Cookie:
		return Cookies{
			cook,
		}, nil
	case http.Cookie:
		return Cookies{
			&cook,
		}, nil
	case Cookies:
		return cook, nil
	case []*http.Cookie:
		return Cookies(cook), nil
	case string:
		return readCookies(http.Header{"Cookie": []string{cook}}, ""), nil
	case http.Header:
		return readCookies(cook, ""), nil
	case []string:
		return readCookies(http.Header{"Cookie": cook}, ""), nil
	default:
		return any2Cookies(cook)
	}
}

func ReadSetCookies(val any) (Cookies, error) {
	switch cook := val.(type) {
	case Cookies:
		return cook, nil
	case []*http.Cookie:
		return Cookies(cook), nil
	case string:
		return readSetCookies(http.Header{"Set-Cookie": []string{cook}}), nil
	case http.Header:
		return readSetCookies(cook), nil
	case []string:
		return readSetCookies(http.Header{"Set-Cookie": cook}), nil
	default:
		return any2Cookies(cook)
	}
}
func any2Cookies(val any) (Cookies, error) {
	switch cooks := val.(type) {
	case map[string]string:
		cookies := Cookies{}
		for kk, vv := range cooks {
			cookies = append(cookies, &http.Cookie{
				Name:  kk,
				Value: vv,
			})
		}
		return cookies, nil
	case map[string][]string:
		cookies := Cookies{}
		for kk, vvs := range cooks {
			for _, vv := range vvs {
				cookies = append(cookies, &http.Cookie{
					Name:  kk,
					Value: vv,
				})
			}
		}
		return cookies, nil
	case *gson.Client:
		if !cooks.IsObject() {
			return nil, errors.New("cookies不支持的类型")
		}
		cookies := Cookies{}
		for kk, vvs := range cooks.Map() {
			if vvs.IsArray() {
				for _, vv := range vvs.Array() {
					cookies = append(cookies, &http.Cookie{
						Name:  kk,
						Value: vv.String(),
					})
				}
			} else {
				cookies = append(cookies, &http.Cookie{
					Name:  kk,
					Value: vvs.String(),
				})
			}
		}
		return cookies, nil
	default:
		jsonData, err := gson.Decode(cooks)
		if err != nil {
			return nil, err
		}
		cookies := Cookies{}
		for kk, vvs := range jsonData.Map() {
			if vvs.IsArray() {
				for _, vv := range vvs.Array() {
					cookies = append(cookies, &http.Cookie{
						Name:  kk,
						Value: vv.String(),
					})
				}
			} else {
				cookies = append(cookies, &http.Cookie{
					Name:  kk,
					Value: vvs.String(),
				})
			}
		}
		return cookies, nil
	}
}
func (obj *RequestOption) initCookies() (err error) {
	if obj.Cookies == nil {
		return nil
	}
	obj.Cookies, err = ReadCookies(obj.Cookies)
	return err
}
