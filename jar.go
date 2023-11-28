package requests

import (
	"errors"
	"net/http/cookiejar"
	"net/url"

	"github.com/gospider007/gtls"
	"golang.org/x/net/publicsuffix"
)

// cookies jar
type Jar struct {
	jar *cookiejar.Jar
}

// new cookies jar
func NewJar() *Jar {
	jar, _ := cookiejar.New(nil)
	return &Jar{
		jar: jar,
	}
}

// get cookies
func (obj *Client) GetCookies(href string) (Cookies, error) {
	return obj.option.Jar.GetCookies(href)
}

// set cookies
func (obj *Client) SetCookies(href string, cookies ...any) error {
	return obj.option.Jar.SetCookies(href, cookies...)
}

// clear cookies
func (obj *Client) ClearCookies() {
	if obj.client.Jar != nil {
		obj.option.Jar.ClearCookies()
		obj.client.Jar = obj.option.Jar.jar
	}
}

// Get cookies
func (obj *Jar) GetCookies(href string) (Cookies, error) {
	if obj.jar == nil {
		return nil, errors.New("jar is nil")
	}
	u, err := url.Parse(href)
	if err != nil {
		return nil, err
	}
	return obj.jar.Cookies(u), nil
}

// Set cookies
func (obj *Jar) SetCookies(href string, cookies ...any) error {
	if obj.jar == nil {
		return errors.New("jar is nil")
	}
	u, err := url.Parse(href)
	if err != nil {
		return err
	}
	domain := u.Hostname()
	if _, addType := gtls.ParseHost(domain); addType == 0 {
		if domain, err = publicsuffix.EffectiveTLDPlusOne(domain); err != nil {
			return err
		}
	}
	for _, cookie := range cookies {
		cooks, err := ReadCookies(cookie)
		if err != nil {
			return err
		}
		for _, cook := range cooks {
			cook.Path = "/"
			cook.Domain = domain
		}
		obj.jar.SetCookies(u, cooks)
	}
	return nil
}

// Clear cookies
func (obj *Jar) ClearCookies() {
	jar, _ := cookiejar.New(nil)
	obj.jar = jar
}
