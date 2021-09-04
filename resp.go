package req

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"os"
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
