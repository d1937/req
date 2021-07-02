package req

import (
	"fmt"
	"testing"
)

func TestGetEncoding(t *testing.T) {
	resp, err := Get("https://www.baidu.com/")
	if resp != nil {
		defer resp.Close()
	}
	if err != nil {
		t.Fatal(err.Error())
	}
	content := resp.Bytes()
	//fmt.Println(string(content))
	html := string(content)
	// fmt.Println(html)
	encoding := GetEncoding(content, resp.Response().Header)

	if encoding != "" {
		html = EncodingConvertToUtf8(html, encoding)
	}
	fmt.Println(html)
}
