package req

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sync"
	"time"
)

// Resp represents a request with it's response
type Resp struct {
	r      *Req
	req    *http.Request
	resp   *http.Response
	client *http.Client
	cost   time.Duration
	*multipartHelper
	reqBody  []byte
	respBody []byte
	err      error // delayed error
}

var bytesNewBufferpool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 4096))
	},
}

// Request returns *http.Request
func (r *Resp) Request() *http.Request {
	return r.req
}

// Response returns *http.Response
func (r *Resp) Response() *http.Response {
	return r.resp
}

// Bytes returns response body as []byte
func (r *Resp) Bytes() []byte {
	data, _ := r.ToBytes()
	return data
}

func (r *Resp) Close() {
	if r.resp.Body != nil {
		//_, _ = io.Copy(ioutil.Discard, r.resp.Body)
		err := r.resp.Body.Close()
		if err != nil {
			return
		}

	}

}

func (r *Resp) ToBytes() ([]byte, error) {

	//defer r.Close()
	if r.err != nil {
		return nil, r.err
	}

	if r.respBody != nil {
		return r.respBody, nil
	}
	var reader io.ReadCloser
	var err error
	//Accept-Encoding
	encoding := r.resp.Header.Get("Content-Encoding")
	if encoding == "" {
		encoding = r.resp.Header.Get("Accept-Encoding")
	}
	switch encoding {
	case "gzip", "gzip, deflate":
		if reader, err = gzip.NewReader(r.resp.Body); err != nil {
			return nil, err
		}
	case "deflate":
		if reader, err = zlib.NewReader(r.resp.Body); err != nil {
			return nil, err
		}
	default:
		reader = r.resp.Body
	}

	defer reader.Close()

	b := bytesNewBufferpool.Get().(*bytes.Buffer)
	b.Reset()
	defer bytesNewBufferpool.Put(b)

	_, err = io.Copy(b, reader)
	if err != nil {

		return nil, err
	}
	r.respBody = b.Bytes()
	return r.respBody, nil
}

func (r *Resp) LimitBytes(n int64) ([]byte, error) {

	data, err := r.LimitReaderBytes(n)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Resp) LimitReaderBytes(n int64) ([]byte, error) {

	//defer r.Close()
	if r.err != nil {
		return nil, r.err
	}

	if r.respBody != nil {
		return r.respBody, nil
	}

	b := bytesNewBufferpool.Get().(*bytes.Buffer)
	b.Reset()
	defer bytesNewBufferpool.Put(b)

	lr := io.LimitReader(r.resp.Body, n)

	_, err := io.Copy(b, lr)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// String returns response body as string
func (r *Resp) String() string {
	data, _ := r.ToBytes()
	return string(data)
}

func (resp *Resp) URL() (*url.URL, error) {
	u := resp.req.URL
	switch resp.Response().StatusCode {
	case http.StatusMovedPermanently, http.StatusFound,
		http.StatusSeeOther, http.StatusTemporaryRedirect:
		location, err := resp.Response().Location()
		if err != nil {
			return nil, err
		}
		u = u.ResolveReference(location)
	}
	return u, nil
}

// ToString returns response body as string,
// return error if error happend when reading
// the response body
func (r *Resp) ToString() (string, error) {
	data, err := r.ToBytes()
	return string(data), err
}

// ToJSON convert json response body to struct or map
func (r *Resp) ToJSON(v interface{}) error {
	data, err := r.ToBytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// ToXML convert xml response body to struct or map
func (r *Resp) ToXML(v interface{}) error {
	data, err := r.ToBytes()
	if err != nil {
		return err
	}
	return xml.Unmarshal(data, v)
}

// ToFile download the response body to file with optional download callback
func (r *Resp) ToFile(name string) error {
	//TODO set name to the suffix of url path if name == ""
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()

	if r.respBody != nil {
		_, err = file.Write(r.respBody)
		return err
	}

	defer r.resp.Body.Close()
	_, err = io.Copy(file, r.resp.Body)
	return err
}

func (r *Resp) download(file *os.File) error {
	p := make([]byte, 1024)
	b := r.resp.Body
	defer b.Close()
	var current int64
	for {
		l, err := b.Read(p)
		if l > 0 {
			_, _err := file.Write(p[:l])
			if _err != nil {
				return _err
			}
			current += int64(l)

		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

var regNewline = regexp.MustCompile(`\n|\r`)

func (r *Resp) autoFormat(s fmt.State) {
	req := r.req
	if r.r.flag&Lcost != 0 {
		fmt.Fprint(s, req.Method, " ", req.URL.String(), " ", r.cost)
	} else {
		fmt.Fprint(s, req.Method, " ", req.URL.String())
	}

	// test if it is should be outputed pretty
	var pretty bool
	var parts []string
	addPart := func(part string) {
		if part == "" {
			return
		}
		parts = append(parts, part)
		if !pretty && regNewline.MatchString(part) {
			pretty = true
		}
	}
	if r.r.flag&LreqBody != 0 { // request body
		addPart(string(r.reqBody))
	}
	if r.r.flag&LrespBody != 0 { // response body
		addPart(r.String())
	}

	for _, part := range parts {
		if pretty {
			fmt.Fprint(s, "\n")
		}
		fmt.Fprint(s, " ", part)
	}
}

func (r *Resp) miniFormat(s fmt.State) {
	req := r.req
	if r.r.flag&Lcost != 0 {
		fmt.Fprint(s, req.Method, " ", req.URL.String(), " ", r.cost)
	} else {
		fmt.Fprint(s, req.Method, " ", req.URL.String())
	}
	if r.r.flag&LreqBody != 0 && len(r.reqBody) > 0 { // request body
		str := regNewline.ReplaceAllString(string(r.reqBody), " ")
		fmt.Fprint(s, " ", str)
	}
	if r.r.flag&LrespBody != 0 && r.String() != "" { // response body
		str := regNewline.ReplaceAllString(r.String(), " ")
		fmt.Fprint(s, " ", str)
	}
}

// Format fort the response
func (r *Resp) Format(s fmt.State, verb rune) {
	if r == nil || r.req == nil {
		return
	}
	if s.Flag('+') { // include header and format pretty.
		fmt.Fprint(s, r.Dump())
	} else if s.Flag('-') { // keep all informations in one line.
		r.miniFormat(s)
	} else { // auto
		r.autoFormat(s)
	}
}
