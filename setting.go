package req

import (
	"crypto/tls"
	"errors"
	"golang.org/x/net/publicsuffix"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

var Timeout int = 15

var (
	httpTransport *http.Transport
)

func init() {
	httpTransport = &http.Transport{
		TLSClientConfig: &tls.Config{
			Renegotiation:      tls.RenegotiateOnceAsClient,
			InsecureSkipVerify: true},
		//Dial: func(netw, addr string) (net.Conn, error) {
		//	c, err := net.DialTimeout(netw, addr, time.Second*15) //设置建立连接超时
		//	if err != nil {
		//		return nil, err
		//	}
		//	_ = c.SetDeadline(time.Now().Add(15 * time.Second)) //设置发送接收数据超时
		//	return c, nil
		//},
		DisableKeepAlives:   true,
		MaxIdleConnsPerHost: 1,
		MaxIdleConns:        1,
	}
}
func newClient() *http.Client {
	options := cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	}
	jar, _ := cookiejar.New(&options)
	return &http.Client{
		Jar:       jar,
		Transport: httpTransport,
		Timeout:   time.Duration(Timeout) * time.Second,
	}
}

// Client return the default underlying http client
func (r *Req) Client() *http.Client {
	if r.client == nil {
		r.client = newClient()
	}
	return r.client
}

// SetClient sets the underlying http.Client.
func (r *Req) SetClient(client *http.Client) {
	r.client = client // use default if client == nil
}

func (r *Req) getTransport() *http.Transport {
	trans, _ := r.Client().Transport.(*http.Transport)
	return trans
}

// EnableInsecureTLS allows insecure https
func (r *Req) EnableInsecureTLS(enable bool) {
	trans := r.getTransport()
	if trans == nil {
		return
	}
	if trans.TLSClientConfig == nil {
		trans.TLSClientConfig = &tls.Config{}
	}
	trans.TLSClientConfig.InsecureSkipVerify = enable
}

// EnableCookieenable or disable cookie manager
func (r *Req) EnableCookie(enable bool) {
	if enable {
		jar, _ := cookiejar.New(nil)
		r.Client().Jar = jar
	} else {
		r.Client().Jar = nil
	}
}

// SetTimeout sets the timeout for every request
func (r *Req) SetTimeout(d time.Duration) {
	r.Client().Timeout = d
}

// SetProxyUrl set the simple proxy with fixed proxy url
func (r *Req) SetProxyUrl(rawurl string) error {
	trans := r.getTransport()
	if trans == nil {
		return errors.New("req: no transport")
	}
	u, err := url.Parse(rawurl)
	if err != nil {
		return err
	}
	trans.Proxy = http.ProxyURL(u)
	return nil
}

// SetProxy sets the proxy for every request
func (r *Req) SetProxy(proxy func(*http.Request) (*url.URL, error)) error {
	trans := r.getTransport()
	if trans == nil {
		return errors.New("req: no transport")
	}
	trans.Proxy = proxy
	return nil
}

type jsonEncOpts struct {
	indentPrefix string
	indentValue  string
	escapeHTML   bool
}

func (r *Req) getJSONEncOpts() *jsonEncOpts {
	if r.jsonEncOpts == nil {
		r.jsonEncOpts = &jsonEncOpts{escapeHTML: true}
	}
	return r.jsonEncOpts
}

// In non-HTML settings where the escaping interferes with the readability
// of the output, SetEscapeHTML(false) disables this behavior.
func (r *Req) SetJSONEscapeHTML(escape bool) {
	opts := r.getJSONEncOpts()
	opts.escapeHTML = escape
}

// SetJSONEscapeHTML specifies whether problematic HTML characters
// should be escaped inside JSON quoted strings.
// The default behavior is to escape &, <, and > to \u0026, \u003c, and \u003e
// to avoid certain safety problems that can arise when embedding JSON in HTML.
//
// In non-HTML settings where the escaping interferes with the readability
// of the output, SetEscapeHTML(false) disables this behavior.
//func SetJSONEscapeHTML(escape bool) {
//	std.SetJSONEscapeHTML(escape)
//}

// SetJSONIndent instructs the encoder to format each subsequent encoded
// value as if indented by the package-level function Indent(dst, src, prefix, indent).
// Calling SetIndent("", "") disables indentation.
func (r *Req) SetJSONIndent(prefix, indent string) {
	opts := r.getJSONEncOpts()
	opts.indentPrefix = prefix
	opts.indentValue = indent
}

// SetJSONIndent instructs the encoder to format each subsequent encoded
// value as if indented by the package-level function Indent(dst, src, prefix, indent).
// Calling SetIndent("", "") disables indentation.
//func SetJSONIndent(prefix, indent string) {
//	std.SetJSONIndent(prefix, indent)
//}

type xmlEncOpts struct {
	prefix string
	indent string
}

func (r *Req) getXMLEncOpts() *xmlEncOpts {
	if r.xmlEncOpts == nil {
		r.xmlEncOpts = &xmlEncOpts{}
	}
	return r.xmlEncOpts
}

// SetXMLIndent sets the encoder to generate XML in which each element
// begins on a new indented line that starts with prefix and is followed by
// one or more copies of indent according to the nesting depth.
func (r *Req) SetXMLIndent(prefix, indent string) {
	opts := r.getXMLEncOpts()
	opts.prefix = prefix
	opts.indent = indent
}
