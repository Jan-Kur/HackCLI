package utils

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"golang.org/x/net/publicsuffix"
)

type Transport struct {
	tr        http.RoundTripper
	BeforeReq func(req *http.Request)
	AfterReq  func(resp *http.Response, req *http.Request)
}

func (t *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if t.BeforeReq != nil {
		t.BeforeReq(req)
	}
	resp, err = t.tr.RoundTrip(req)
	if err != nil {
		return
	}
	if t.AfterReq != nil {
		t.AfterReq(resp, req)
	}
	return
}

func NewTransport(tr http.RoundTripper) *Transport {
	t := &Transport{}
	if tr == nil {
		tr = http.DefaultTransport
	}
	t.tr = tr
	return t
}

func NewWithTransport(cookieDomain string, cookies []*http.Cookie, rt http.RoundTripper) *http.Client {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	url, err := url.Parse(cookieDomain)
	if err != nil {
		panic(err)
	}
	jar.SetCookies(url, cookies)
	cl := http.Client{
		Jar:       jar,
		Transport: rt,
	}
	return &cl
}

func NewCookieHTTP(cookieDomain string, cookies []*http.Cookie) *http.Client {
	return NewWithTransport(cookieDomain, cookies, NewTransport(nil))
}

func sliceOfPtr[T any](cc []T) []*T {
	var ret = make([]*T, len(cc))
	for i := range cc {
		ret[i] = &cc[i]
	}
	return ret
}

func ConvertCookies(cc []http.Cookie) []*http.Cookie {
	return sliceOfPtr(cc)
}
