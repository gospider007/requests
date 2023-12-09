package requests

import (
	"net/http/cookiejar"
	"net/url"

	"github.com/gospider007/gtls"
	"golang.org/x/net/publicsuffix"
)

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
	if obj.option.Jar == nil {
		return nil
	}
	return obj.option.Jar.GetCookies(href)
}

// set cookies
func (obj *Client) SetCookies(href *url.URL, cookies ...any) error {
	if obj.option.Jar == nil {
		return nil
	}
	return obj.option.Jar.SetCookies(href, cookies...)
}

// clear cookies
func (obj *Client) ClearCookies() {
	if obj.option.Jar == nil {
		return
	}
	obj.option.Jar.ClearCookies()
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

// Set cookies
func (obj *jar) SetCookies(u *url.URL, cookies ...any) error {
	domain := getDomain(u)
	for _, cookie := range cookies {
		cooks, err := ReadCookies(cookie)
		if err != nil {
			return err
		}
		for _, cook := range cooks {
			if cook.Path == "" {
				cook.Path = "/"
			}
			if cook.Domain == "" {
				cook.Domain = domain
			}
		}
		obj.jar.SetCookies(u, cooks)
	}
	return nil
}

// Clear cookies
func (obj *jar) ClearCookies() {
	jar, _ := cookiejar.New(nil)
	obj.jar = jar
}
