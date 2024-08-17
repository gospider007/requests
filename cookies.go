package requests

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gospider007/gson"
)

// cookies
type Cookies []*http.Cookie

// return cookies with string,join with '; '
func (obj Cookies) String() string {
	cooks := []string{}
	for _, cook := range obj {
		cooks = append(cooks, fmt.Sprintf("%s=%s", cook.Name, cook.Value))
	}
	return strings.Join(cooks, "; ")
}

// get cookies by name
func (obj Cookies) Gets(name string) Cookies {
	var result Cookies
	for _, cook := range obj {
		if cook.Name == name {
			result = append(result, cook)
		}
	}
	return result
}

// get cookie by name
func (obj Cookies) Get(name string) *http.Cookie {
	vals := obj.Gets(name)
	if i := len(vals); i == 0 {
		return nil
	} else {
		return vals[i-1]
	}
}

// get cookie values by name, return []string
func (obj Cookies) GetVals(name string) []string {
	var result []string
	for _, cook := range obj {
		if cook.Name == name {
			result = append(result, cook.Value)
		}
	}
	return result
}

// get cookie value by name,return string
func (obj Cookies) GetVal(name string) string {
	vals := obj.GetVals(name)
	if i := len(vals); i == 0 {
		return ""
	} else {
		return vals[i-1]
	}
}
func (obj Cookies) append(cook *http.Cookie) Cookies {
	return append(obj, cook)
}

// read cookies or parse cookies,support json,map,[]string,http.Header,string
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
		return http.ParseCookie(cook)
	case http.Header:
		return nil, errors.New("cookies not support type")
	case []string:
		return nil, errors.New("cookies not support type")
	default:
		return any2Cookies(cook)
	}
}

// read set cookies or parse set cookies,support json,map,[]string,http.Header,string
//
//	func ReadSetCookies(val any) (Cookies, error) {
//		switch cook := val.(type) {
//		case Cookies:
//			return cook, nil
//		case []*http.Cookie:
//			return Cookies(cook), nil
//		case string:
//			http.ParseCookie()
//			return http.ParseSetCookie(cook)
//		case http.Header:
//			return readSetCookies(cook), nil
//		case []string:
//			return readSetCookies(http.Header{"Set-Cookie": cook}), nil
//		default:
//			return any2Cookies(cook)
//		}
//	}
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
			return nil, errors.New("cookies not support type")
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
func (obj *RequestOption) initCookies() (Cookies, error) {
	if obj.Cookies == nil {
		return nil, nil
	}
	return ReadCookies(obj.Cookies)
}
