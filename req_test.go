package req

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestUrlParam(t *testing.T) {
	m := map[string]interface{}{
		"access_token": "123abc",
		"name":         "roc",
		"enc":          "中文",
	}
	r := New()
	queryHandler := func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		for key, value := range m {
			if v := query.Get(key); value != v {
				t.Errorf("query param %s = %s; want = %s", key, v, value)
			}
		}
	}
	ts := httptest.NewServer(http.HandlerFunc(queryHandler))
	_, err := r.Get(ts.URL, QueryParam(m))
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Head(ts.URL, Param(m))
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Put(ts.URL, QueryParam(m))
	if err != nil {
		t.Fatal(err)
	}
}

func TestFormParam(t *testing.T) {
	formParam := Param{
		"access_token": "123abc",
		"name":         "roc",
		"enc":          "中文",
	}
	r := New()
	formHandler := func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		for key, value := range formParam {
			if v := r.FormValue(key); value != v {
				t.Errorf("form param %s = %s; want = %s", key, v, value)
			}
		}
	}
	ts := httptest.NewServer(http.HandlerFunc(formHandler))
	url := ts.URL
	_, err := r.Post(url, formParam)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParamBoth(t *testing.T) {
	urlParam := QueryParam{
		"access_token": "123abc",
		"enc":          "中文",
	}
	formParam := Param{
		"name": "roc",
		"job":  "软件工程师",
	}
	r := New()
	handler := func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		for key, value := range urlParam {
			if v := query.Get(key); value != v {
				t.Errorf("query param %s = %s; want = %s", key, v, value)
			}
		}
		_ = r.ParseForm()
		for key, value := range formParam {
			if v := r.FormValue(key); value != v {
				t.Errorf("form param %s = %s; want = %s", key, v, value)
			}
		}
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	url := ts.URL
	_, err := r.Patch(url, urlParam, formParam)
	if err != nil {
		t.Fatal(err)
	}

}

func TestBody(t *testing.T) {
	body := "request body"
	handler := func(w http.ResponseWriter, r *http.Request) {
		bs, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(bs) != body {
			t.Errorf("body = %s; want = %s", bs, body)
		}
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	r := New()
	// string
	_, err := r.Post(ts.URL, body)
	if err != nil {
		t.Fatal(err)
	}

	// []byte
	_, err = r.Post(ts.URL, []byte(body))
	if err != nil {
		t.Fatal(err)
	}

	// *bytes.Buffer
	var buf bytes.Buffer
	buf.WriteString(body)
	_, err = r.Post(ts.URL, &buf)
	if err != nil {
		t.Fatal(err)
	}

	// io.Reader
	_, err = r.Post(ts.URL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
}

func TestPostJson(t *testing.T) {
	Debug = true
	jsonstr := `{
  "name":"samy",
  "age":"19"
	}`
	r := New()
	resp, err := r.Post("https://httpbin.org/post", BodyJSON(&jsonstr))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(resp.String())
}

func TestPostXml(t *testing.T) {
	Debug = true
	xmlstr := `
		<?xml version="1.0" encoding="UTF-8"?>
<custom key="12321321">
  <document name="sample" location="http://www.sample.com"></document>
</custom>
		`
	r := New()
	//r.SetJSONEscapeHTML(false)
	//r.SetJSONIndent("", "\t")
	resp, err := r.Post("https://reqbin.com/echo/post/xml", BodyXML(&xmlstr))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(resp.String())

}

func TestBodyJSON(t *testing.T) {
	type content struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	c := content{
		Code: 1,
		Msg:  "ok",
	}
	checkData := func(data []byte) {
		var cc content
		err := json.Unmarshal(data, &cc)
		if err != nil {
			t.Fatal(err)
		}
		if cc != c {
			t.Errorf("request body = %+v; want = %+v", cc, c)
		}
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		checkData(data)
	})
	r := New()
	ts := httptest.NewServer(handler)
	resp, err := r.Post(ts.URL, BodyJSON(&c))
	if err != nil {
		t.Fatal(err)
	}
	checkData(resp.reqBody)

	//SetJSONEscapeHTML(false)
	//SetJSONIndent("", "\t")
	resp, err = r.Put(ts.URL, BodyJSON(&c))
	if err != nil {
		t.Fatal(err)
	}
	checkData(resp.reqBody)
}

func TestBodyXML(t *testing.T) {
	type content struct {
		Code int    `xml:"code"`
		Msg  string `xml:"msg"`
	}
	c := content{
		Code: 1,
		Msg:  "ok",
	}
	checkData := func(data []byte) {
		var cc content
		err := xml.Unmarshal(data, &cc)
		if err != nil {
			t.Fatal(err)
		}
		if cc != c {
			t.Errorf("request body = %+v; want = %+v", cc, c)
		}
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		checkData(data)
	})
	r := New()
	ts := httptest.NewServer(handler)
	resp, err := r.Post(ts.URL, BodyXML(&c))
	if err != nil {
		t.Fatal(err)
	}
	checkData(resp.reqBody)

	//SetXMLIndent("", "    ")
	resp, err = r.Put(ts.URL, BodyXML(&c))
	if err != nil {
		t.Fatal(err)
	}
	checkData(resp.reqBody)
}

func TestHeader(t *testing.T) {
	header := Header{
		"User-Agent":    "V1.0.0",
		"Authorization": "roc",
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		for key, value := range header {
			if v := r.Header.Get(key); value != v {
				t.Errorf("header %q = %s; want = %s", key, v, value)
			}
		}
	}
	r := New()
	ts := httptest.NewServer(http.HandlerFunc(handler))
	_, err := r.Head(ts.URL, header)
	if err != nil {
		t.Fatal(err)
	}

	httpHeader := make(http.Header)
	for key, value := range header {
		httpHeader.Add(key, value)
	}
	_, err = r.Head(ts.URL, httpHeader)
	if err != nil {
		t.Fatal(err)
	}
}

//func TestUpload(t *testing.T) {
//	str := "hello req"
//	file := ioutil.NopCloser(strings.NewReader(str))
//	upload := FileUpload{
//		File:      file,
//		FieldName: "media",
//		FileName:  "hello.txt",
//	}
//	handler := func(w http.ResponseWriter, r *http.Request) {
//		mr, err := r.MultipartReader()
//		if err != nil {
//			t.Fatal(err)
//		}
//		for {
//			p, err := mr.NextPart()
//			if err != nil {
//				break
//			}
//			if p.FileName() != upload.FileName {
//				t.Errorf("filename = %s; want = %s", p.FileName(), upload.FileName)
//			}
//			if p.FormName() != upload.FieldName {
//				t.Errorf("formname = %s; want = %s", p.FileName(), upload.FileName)
//			}
//			data, err := ioutil.ReadAll(p)
//			if err != nil {
//				t.Fatal(err)
//			}
//			if string(data) != str {
//				t.Errorf("file content = %s; want = %s", data, str)
//			}
//		}
//	}
//	ts := httptest.NewServer(http.HandlerFunc(handler))
//	r := New()
//	_, err := r.Post(ts.URL, upload)
//	if err != nil {
//		t.Fatal(err)
//	}
//	ts = newDefaultTestServer()
//	_, err = r.Post(ts.URL, File("*.go"))
//	if err != nil {
//		t.Fatal(err)
//	}
//}

func TestReq_Get(t *testing.T) {
	r := New()
	resp, err := r.Get("https://www.baidu.com/")
	if err != nil {
		t.Fatal(err.Error())
	}

	fmt.Println(resp.String())
}

func TestReq_AddCookies(t *testing.T) {
	r := New()
	cookies := make(map[string]string)
	cookies["name"] = "samy"
	cookies["age"] = "18"
	r.AddCookies(cookies)
	resp, err := r.Get("http://127.0.0.1/cookies/set")
	if resp != nil {
		defer resp.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(resp.String())
	fmt.Println()
	cookies["name"] = "jacks"
	cookies["age"] = "19"
	cookies["addr"] = "asfd"
	r.UpdateCookie(cookies)
	resp, err = r.Get("http://127.0.0.1/cookies/set")
	if resp != nil {
		defer resp.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(resp.String())
}

func TestReq_SetBasicAuth(t *testing.T) {
	r := New()
	r.SetBasicAuth("admin", "admin")
	resp, err := r.Get("http://127.0.0.1/basic-auth/admin/admin")
	if resp != nil {
		defer resp.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(resp.String())
}

const (
	HttpProxy  = "http://127.0.0.1:1378"
	SocksProxy = "socks5://127.0.0.1:1378"
)

func TestReq_Upload(t *testing.T) {
	r := New()
	Debug = true
	//str := "hello req"
	//file := ioutil.NopCloser(strings.NewReader(str))
	file, err := os.Open("C:\\Users\\samy1\\Desktop\\a\\a.png")
	if err != nil {
		t.Fatal(err)
	}
	upload := FileUpload{
		File:        file,
		FieldName:   "file",
		FileName:    "aasdf.php",
		ContentType: "image/jpeg",
	}

	// r.SetProxyUrl("http://127.0.0.1:8080")

	args := Param{
		"submit": "",
	}

	resp, err := r.Post("http://ttt.com/upload_file.php", upload, args)

	if resp != nil {
		defer resp.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(resp.String())
}

func TestReq_Options(t *testing.T) {
	r := New()
	resp, err := r.Options("https://www.zhuzhou.com/")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(resp.String())
}
