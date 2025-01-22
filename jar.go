package requests

import (
	"net/http/cookiejar"
	"net/url"

	"github.com/gospider007/gtls"
	"golang.org/x/net/publicsuffix"
)

type Jar interface { // size=8
	ClearCookies()
	GetCookies(u *url.URL) Cookies
	SetCookies(*url.URL, Cookies)
}

// cookies jar
type jar struct {
	jar *cookiejar.Jar
}

// new cookies jar
func NewJar() *jar {
	j, _ := cookiejar.New(nil)
	return &jar{
		jar: j,
	}
}

// get cookies
func (obj *Client) GetCookies(href *url.URL) Cookies {
	if obj.ClientOption.Jar == nil {
		return nil
	}
	return obj.ClientOption.Jar.GetCookies(href)
}

// set cookies
func (obj *Client) SetCookies(href *url.URL, cookies ...any) error {
	if obj.ClientOption.Jar == nil {
		return nil
	}
	cooks, err := any2cookies(href, cookies...)
	if err != nil {
		return err
	}
	obj.ClientOption.Jar.SetCookies(href, cooks)
	return nil
}

// clear cookies
func (obj *Client) ClearCookies() {
	if obj.ClientOption.Jar == nil {
		return
	}
	obj.ClientOption.Jar.ClearCookies()
}

// Get cookies
func (obj *jar) GetCookies(u *url.URL) Cookies {
	return obj.jar.Cookies(u)
}
func getDomain(u *url.URL) string {
	domain := u.Hostname()
	if _, addType := gtls.ParseHost(domain); addType == 0 {
		if tlp, err := publicsuffix.EffectiveTLDPlusOne(domain); err == nil {
			domain = tlp
		}
	}
	return domain
}
func any2cookies(u *url.URL, cookies ...any) (Cookies, error) {
	domain := getDomain(u)
	var result Cookies
	for _, cookie := range cookies {
		cooks, err := ReadCookies(cookie)
		if err != nil {
			return cooks, err
		}
		for _, cook := range cooks {
			if cook.Path == "" {
				cook.Path = "/"
			}
			if cook.Domain == "" {
				cook.Domain = domain
			}
		}
		result = append(result, cooks...)
	}
	return result, nil
}

// Set cookies
func (obj *jar) SetCookies(u *url.URL, cookies Cookies) {
	obj.jar.SetCookies(u, cookies)
}

// Clear cookies
func (obj *jar) ClearCookies() {
	jar, _ := cookiejar.New(nil)
	obj.jar = jar
}
