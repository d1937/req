package req

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	LreqHead  = 1 << iota // output request head (request line and request header)
	LreqBody              // output request body
	LrespHead             // output response head (response line and response header)
	LrespBody             // output response body
	Lcost                 // output time costed by the request
	LstdFlags = LreqHead | LreqBody | LrespHead | LrespBody
)

type BasicAuth struct {
	Username string
	Password string
}

// Param represents  http request param
type Param map[string]interface{}

// QueryParam is used to force append http request param to the uri
type QueryParam map[string]interface{}

// Host is used for set request's Host
type Host string

// FileUpload represents a file to upload
type FileUpload struct {
	// filename in multipart form.
	FileName string
	// form field name
	FieldName string
	// file to uplaod, required
	File        io.ReadCloser
	ContentType string
}

type DownloadProgress func(current, total int64)

type UploadProgress func(current, total int64)

type FormData string

type AllowRedirects bool

// File upload files matching the name pattern such as
// /usr/*/bin/go* (assuming the Separator is '/')
func File(patterns ...string) interface{} {
	matches := []string{}
	for _, pattern := range patterns {
		m, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		matches = append(matches, m...)
	}
	if len(matches) == 0 {
		return errors.New("req: no file have been matched")
	}
	uploads := []FileUpload{}
	for _, match := range matches {
		if s, e := os.Stat(match); e != nil || s.IsDir() {
			continue
		}
		file, _ := os.Open(match)
		uploads = append(uploads, FileUpload{
			File:      file,
			FileName:  filepath.Base(match),
			FieldName: "media",
		})
	}

	return uploads
}

type bodyJson struct {
	v interface{}
}

type bodyXml struct {
	v interface{}
}

// BodyJSON make the object be encoded in json format and set it to the request body
func BodyJSON(v interface{}) *bodyJson {
	return &bodyJson{v: v}
}

// BodyXML make the object be encoded in xml format and set it to the request body
func BodyXML(v interface{}) *bodyXml {
	return &bodyXml{v: v}
}

// Req is a convenient client for initiating requests
type Req struct {
	client           *http.Client
	jsonEncOpts      *jsonEncOpts
	xmlEncOpts       *xmlEncOpts
	progressInterval time.Duration
	Req              *http.Request
	flag             int
}

// New create a new *Req
func New() *Req {
	// default progress reporting interval is 200 milliseconds
	req := &http.Request{
		//Method:     method,

		Header:     make(http.Header),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	return &Req{Req: req, client: newClient(), flag: LstdFlags}
}

type param struct {
	url.Values
}

func (p *param) getValues() url.Values {
	if p.Values == nil {
		p.Values = make(url.Values)
	}
	return p.Values
}

func (p *param) Copy(pp param) {
	if pp.Values == nil {
		return
	}
	vs := p.getValues()
	for key, values := range pp.Values {
		for _, value := range values {
			vs.Add(key, value)
		}
	}
}
func (p *param) Adds(m map[string]interface{}) {
	if len(m) == 0 {
		return
	}
	vs := p.getValues()
	for k, v := range m {
		vs.Add(k, fmt.Sprint(v))
	}
}

func (p *param) Empty() bool {
	return p.Values == nil
}

// Do execute a http request with sepecify method and url,
// and it can also have some optional params, depending on your needs.
func (r *Req) Do(method, rawurl string, vs ...interface{}) (resp *Resp, err error) {
	if rawurl == "" {
		return nil, errors.New("req: url not specified")
	}

	//ctx, cancelFunc := context.WithTimeout(context.Background(), 1*time.Minute)
	////ctx, cancelFunc := context.WithCancel(context.Background())
	//defer cancelFunc()
	//req = req.WithContext(ctx)
	//r.Req.Close = true
	resp = &Resp{req: r.Req, r: r}
	r.Req.Method = method

	//allowRedirects := true

	var queryParam param
	var formParam param
	var uploads []FileUpload
	var delayedFunc []func()
	var lastFunc []func()

	for _, v := range vs {
		switch vv := v.(type) {
		case Header:
			for key, value := range vv {
				r.Req.Header.Add(key, value)
			}
		case http.Header:
			for key, values := range vv {
				for _, value := range values {
					r.Req.Header.Add(key, value)
				}
			}
		case BasicAuth:
			r.Req.SetBasicAuth(vv.Username, vv.Password)
		case *bodyJson:
			fn, err := setBodyJson(r.Req, resp, r.jsonEncOpts, vv.v)
			if err != nil {
				return nil, err
			}
			delayedFunc = append(delayedFunc, fn)
		case *bodyXml:
			fn, err := setBodyXml(r.Req, resp, r.xmlEncOpts, vv.v)
			if err != nil {
				return nil, err
			}
			delayedFunc = append(delayedFunc, fn)
		case url.Values:
			p := param{vv}
			if method == "GET" || method == "HEAD" {
				queryParam.Copy(p)
			} else {
				formParam.Copy(p)
			}
		case FormData:
			setBodyBytes(r.Req, resp, []byte(vv))
			r.Req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		case Param:
			if method == "GET" || method == "HEAD" {
				queryParam.Adds(vv)
			} else {
				formParam.Adds(vv)
			}
		case QueryParam:
			queryParam.Adds(vv)
		case string:
			setBodyBytes(r.Req, resp, []byte(vv))
		case []byte:
			setBodyBytes(r.Req, resp, vv)
		case bytes.Buffer:
			setBodyBytes(r.Req, resp, vv.Bytes())
		case *http.Client:
			resp.client = vv
		case FileUpload:
			//if vv.ContentType!="" {
			//	r.Req.Header.Set("Content-Type",vv.ContentType)
			//}

			uploads = append(uploads, vv)
		case []FileUpload:
			uploads = append(uploads, vv...)
		case *http.Cookie:
			r.Req.AddCookie(vv)
		case Host:
			r.Req.Host = string(vv)
		case io.Reader:
			fn := setBodyReader(r.Req, resp, vv)
			lastFunc = append(lastFunc, fn)

		case context.Context:
			r.Req = r.Req.WithContext(vv)
			resp.req = r.Req

		case error:
			return nil, vv
		}
	}
	if r.Req.Header.Get("User-Agent") == "" || r.Req.Header.Get("user-agent") == "" {
		r.Req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/70.0.3538.77 Safari/537.36")
	}

	if length := r.Req.Header.Get("Content-Length"); length != "" {
		if l, err := strconv.ParseInt(length, 10, 64); err == nil {
			r.Req.ContentLength = l
		}
	}

	if len(uploads) > 0 && (r.Req.Method == "POST" || r.Req.Method == "PUT") { // multipart

		multipartHelper := &multipartHelper{
			form:    formParam.Values,
			uploads: uploads,
		}
		multipartHelper.UploadX(r.Req)

		resp.multipartHelper = multipartHelper
	} else {

		if !formParam.Empty() {
			if r.Req.Body != nil {
				queryParam.Copy(formParam)
			} else {
				setBodyBytes(r.Req, resp, []byte(formParam.Encode()))
				setContentType(r.Req, "application/x-www-form-urlencoded; charset=UTF-8")
			}
		}
	}

	if !queryParam.Empty() {
		paramStr := queryParam.Encode()
		if strings.IndexByte(rawurl, '?') == -1 {
			rawurl = rawurl + "?" + paramStr
		} else {
			rawurl = rawurl + "&" + paramStr
		}
	}

	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	r.Req.URL = u

	if host := r.Req.Header.Get("Host"); host != "" {
		r.Req.Host = host
	}

	for _, fn := range delayedFunc {
		fn()
	}

	if resp.client == nil {
		resp.client = r.Client()
	}

	var response *http.Response
	response, err = resp.client.Do(r.Req)
	if err != nil {
		return nil, err
	}

	for _, fn := range lastFunc {
		fn()
	}

	resp.resp = response

	//// output detail if Debug is enabled
	if Debug {
		fmt.Println(resp.Dump())
	}
	return
}

func (r *Req) DisableAllowRedirects() {

	r.Client().CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
}

func setBodyBytes(req *http.Request, resp *Resp, data []byte) {
	resp.reqBody = data
	req.Body = ioutil.NopCloser(bytes.NewReader(data))
	req.ContentLength = int64(len(data))
}

func setBodyJson(req *http.Request, resp *Resp, opts *jsonEncOpts, v interface{}) (func(), error) {
	var data []byte
	switch vv := v.(type) {
	case string:
		data = []byte(vv)
	case []byte:
		data = vv
	case *bytes.Buffer:
		data = vv.Bytes()
	default:
		if opts != nil {
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			enc.SetIndent(opts.indentPrefix, opts.indentValue)
			enc.SetEscapeHTML(opts.escapeHTML)
			err := enc.Encode(v)
			if err != nil {
				return nil, err
			}
			data = buf.Bytes()
		} else {
			var err error
			data, err = json.Marshal(v)
			if err != nil {
				return nil, err
			}
		}
	}
	setBodyBytes(req, resp, data)
	delayedFunc := func() {
		setContentType(req, "application/json; charset=UTF-8")
	}
	return delayedFunc, nil
}

func setBodyXml(req *http.Request, resp *Resp, opts *xmlEncOpts, v interface{}) (func(), error) {
	var data []byte
	switch vv := v.(type) {
	case string:
		data = []byte(vv)
	case []byte:
		data = vv
	case *bytes.Buffer:
		data = vv.Bytes()
	default:
		if opts != nil {
			var buf bytes.Buffer
			enc := xml.NewEncoder(&buf)
			enc.Indent(opts.prefix, opts.indent)
			err := enc.Encode(v)
			if err != nil {
				return nil, err
			}
			data = buf.Bytes()
		} else {
			var err error
			data, err = xml.Marshal(v)
			if err != nil {
				return nil, err
			}
		}
	}
	setBodyBytes(req, resp, data)
	delayedFunc := func() {
		setContentType(req, "application/xml; charset=UTF-8")
	}
	return delayedFunc, nil
}

func setContentType(req *http.Request, contentType string) {
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}
}

func setBodyReader(req *http.Request, resp *Resp, rd io.Reader) func() {
	var rc io.ReadCloser
	switch r := rd.(type) {
	case *os.File:
		stat, err := r.Stat()
		if err == nil {
			req.ContentLength = stat.Size()
		}
		rc = r

	case io.ReadCloser:
		rc = r
	default:
		rc = ioutil.NopCloser(rd)
	}
	bw := &bodyWrapper{
		ReadCloser: rc,
		limit:      102400,
	}
	req.Body = bw
	lastFunc := func() {
		resp.reqBody = bw.buf.Bytes()
	}
	return lastFunc
}

type bodyWrapper struct {
	io.ReadCloser
	buf   bytes.Buffer
	limit int
}

func (b *bodyWrapper) Read(p []byte) (n int, err error) {
	n, err = b.ReadCloser.Read(p)
	if left := b.limit - b.buf.Len(); left > 0 && n > 0 {
		if n <= left {
			b.buf.Write(p[:n])
		} else {
			b.buf.Write(p[:left])
		}
	}
	return
}

type multipartHelper struct {
	form             url.Values
	uploads          []FileUpload
	dump             []byte
	uploadProgress   UploadProgress
	progressInterval time.Duration
	ContentType      string
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func (m *multipartHelper) Upload(req *http.Request) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for key, values := range m.form {
		for _, value := range values {
			_ = m.writeField(w, key, value)
		}
	}
	for _, up := range m.uploads {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
				escapeQuotes(up.FieldName), escapeQuotes(up.FileName)))
		h.Set("Content-Type", "image/jpg")

		p, _ := w.CreatePart(h)

		io.Copy(p, up.File)

	}
	_ = w.Close()

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Body = ioutil.NopCloser(&buf)

}

func (m *multipartHelper) Dump() []byte {
	if m.dump != nil {
		return m.dump
	}
	var buf bytes.Buffer
	bodyWriter := multipart.NewWriter(&buf)
	for key, values := range m.form {
		for _, value := range values {
			_ = m.writeField(bodyWriter, key, value)
		}
	}

	for _, up := range m.uploads {
		_, _ = m.writeFile(bodyWriter, up.FieldName, up.FileName, up.ContentType)

	}
	_ = bodyWriter.Close()
	m.dump = buf.Bytes()
	return m.dump
}

func (m *multipartHelper) UploadX(req *http.Request) {

	var buf bytes.Buffer
	bodyWriter := multipart.NewWriter(&buf)
	for key, values := range m.form {
		for _, value := range values {
			_ = m.writeField(bodyWriter, key, value)
		}
	}

	for _, up := range m.uploads {
		write, err := m.writeFile(bodyWriter, up.FieldName, up.FileName, up.ContentType)
		if err != nil {
			continue
		}
		_, _ = io.Copy(write, up.File)

	}
	_ = bodyWriter.Close()
	m.dump = buf.Bytes()
	req.ContentLength = int64(buf.Len())
	req.Header.Set("Content-Type", bodyWriter.FormDataContentType())
	req.Body = ioutil.NopCloser(&buf)
}

func (m *multipartHelper) writeField(w *multipart.Writer, fieldname, value string) error {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"`, escapeQuotes(fieldname)))
	p, err := w.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = p.Write([]byte(value))
	return err
}

func (m *multipartHelper) writeFile(w *multipart.Writer, fieldname, filename string, contentType string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			fieldname, filename))
	if contentType != "" {
		h.Set("Content-Type", contentType)
	} else {
		h.Set("Content-Type", "application/octet-stream")
	}

	p, err := w.CreatePart(h)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// Get execute a http GET request
func (r *Req) Get(url string, v ...interface{}) (*Resp, error) {
	return r.Do("GET", url, v...)
}

// Post execute a http POST request
func (r *Req) Post(url string, v ...interface{}) (*Resp, error) {
	return r.Do("POST", url, v...)
}

// Put execute a http PUT request
func (r *Req) Put(url string, v ...interface{}) (*Resp, error) {
	return r.Do("PUT", url, v...)
}

// Patch execute a http PATCH request
func (r *Req) Patch(url string, v ...interface{}) (*Resp, error) {
	return r.Do("PATCH", url, v...)
}

// Delete execute a http DELETE request
func (r *Req) Delete(url string, v ...interface{}) (*Resp, error) {
	return r.Do("DELETE", url, v...)
}

// Head execute a http HEAD request
func (r *Req) Head(url string, v ...interface{}) (*Resp, error) {
	return r.Do("HEAD", url, v...)
}

// Options execute a http OPTIONS request
func (r *Req) Options(url string, v ...interface{}) (*Resp, error) {
	return r.Do("OPTIONS", url, v...)
}

func (r *Req) SetBasicAuth(username, password string) {
	r.Req.SetBasicAuth(username, password)
}

func (r *Req) AddCookies(cookieMap map[string]string) {
	if cookieMap == nil {
		return
	}
	for k, v := range cookieMap {
		cookie := &http.Cookie{Name: k, Value: v}
		r.Req.AddCookie(cookie)
	}
}

func (r *Req) AddHeader(headers map[string]string) {
	if headers == nil {
		return
	}

	for k, v := range headers {
		r.Req.Header.Add(k, v)
	}

}

func (r *Req) UpdateCookie(cookies interface{}) {
	switch cookies.(type) {
	case map[string]string:
		if cmap, ok := cookies.(map[string]string); ok {
			for k, v := range cmap {
				cc := &http.Cookie{Name: k, Value: v}
				r.Req.AddCookie(cc)
			}
		}

	case string:
		if cstr, ok := cookies.(string); ok {
			c := strings.Split(cstr, ";")
			for _, value := range c {
				cv := strings.Split(value, "=")
				if len(cv) == 2 {

					cc := &http.Cookie{Name: cv[0], Value: cv[1]}
					r.Req.AddCookie(cc)
				}
			}
		}

	}

}


// Get execute a http GET request
func Get(url string, v ...interface{}) (*Resp, error) {
	r := New()
	return r.Get(url, v...)
}

// Post execute a http POST request
func Post(url string, v ...interface{}) (*Resp, error) {
	r := New()
	return r.Post(url, v...)
}

// Put execute a http PUT request
func Put(url string, v ...interface{}) (*Resp, error) {
	r := New()
	return r.Put(url, v...)
}

// Head execute a http HEAD request
func Head(url string, v ...interface{}) (*Resp, error) {
	r := New()
	return r.Head(url, v...)
}

// Options execute a http OPTIONS request
func Options(url string, v ...interface{}) (*Resp, error) {
	r := New()
	return r.Options(url, v...)
}

// Delete execute a http DELETE request
func Delete(url string, v ...interface{}) (*Resp, error) {
	r := New()
	return r.Delete(url, v...)
}

// Patch execute a http PATCH request
func Patch(url string, v ...interface{}) (*Resp, error) {
	r := New()
	return r.Patch(url, v...)
}

// Do execute request.
func Do(method, url string, v ...interface{}) (*Resp, error) {
	r := New()
	return r.Do(method, url, v...)
}
