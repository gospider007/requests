package requests

import (
	"errors"
	"net/http/cookiejar"
	"net/url"

	"golang.org/x/net/publicsuffix"
)

type Jar struct {
	jar *cookiejar.Jar
}

func NewJar() *Jar {
	jar, _ := cookiejar.New(nil)
	return &Jar{
		jar: jar,
	}
}

func (obj *Client) GetCookies(href string) (Cookies, error) {
	return obj.jar.GetCookies(href)
}
func (obj *Client) SetCookies(href string, cookies ...any) error {
	return obj.jar.SetCookies(href, cookies...)
}

func (obj *Client) ClearCookies() {
	if obj.client.Jar != nil {
		obj.jar.ClearCookies()
		obj.client.Jar = obj.jar.jar
	}
}
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
func (obj *Jar) SetCookies(href string, cookies ...any) error {
	if obj.jar == nil {
		return errors.New("jar is nil")
	}
	u, err := url.Parse(href)
	if err != nil {
		return err
	}
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
				cook.Domain, _ = publicsuffix.PublicSuffix(u.Hostname())
				if err != nil {
					return err
				}
			}
		}
		obj.jar.SetCookies(u, cooks)
	}
	return nil
}
func (obj *Jar) ClearCookies() {
	jar, _ := cookiejar.New(nil)
	obj.jar = jar
}
