package req

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/axgle/mahonia"
	"golang.org/x/net/html/charset"
)

var (
	charsetRe = regexp.MustCompile(`(?is)<meta.*?charset=["\']*(.+?)["\'>]`)
	pragmaRe  = regexp.MustCompile(`(?is)<meta.*?content=["\']*;?charset=(.+?)["\'>]`)
	xmlRe     = regexp.MustCompile(`(?is)^<\?XML.*?encoding=["\']*(.+?)["\'>]`)
	//fmt.Sprintf("^(?%s:%s", "i", pattern[1:]1)
)

func parseContentTypeHeader(header string) (string, map[string]string) {
	tokens := strings.Split(header, ";")
	contentType, params := strings.TrimSpace(tokens[0]), tokens[1:]
	itemsToStrip := "\"' "
	paramsDict := make(map[string]string)
	for _, param := range params {
		param = strings.TrimSpace(param)
		if param != "" {
			index := strings.Index(param, "=")
			if index != -1 {
				//  text/html; charset=utf-8
				key := strings.ToLower(strings.Trim(param[:index], itemsToStrip)) // charset
				value := strings.Trim(param[index+1:], itemsToStrip)              //utf-8
				paramsDict[key] = value

			}
		}
	}

	return contentType, paramsDict
}

func getEncodingFromHeaders(headers http.Header) string {
	contentType := strings.ToLower(headers.Get("Content-Type"))
	if contentType == "" {
		return ""
	}
	contentType, params := parseContentTypeHeader(contentType)
	if v, ok := params["charset"]; ok {
		return strings.Trim(v, "'\\\"")
	}

	if strings.Contains(contentType, "text") {
		return "ISO-8859-1"
	}

	if strings.Contains(contentType, "application/json") {
		return "utf-8"
	}

	return ""
}

func getEncodingFromContent(content string) string {

	if content == "" {
		return ""
	}
	match := charsetRe.FindStringSubmatch(content)
	if len(match) > 1 {
		contentType := strings.ToLower(match[1])
		return contentType
	}

	match = pragmaRe.FindStringSubmatch(content)
	if len(match) > 1 {
		contentType := strings.ToLower(match[1])
		return contentType
	}

	match = xmlRe.FindStringSubmatch(content)
	if len(match) > 1 {
		contentType := strings.ToLower(match[1])
		return contentType
	}
	return ""
}

func GetEncoding(content []byte, headers http.Header) string {

	if c := getEncodingFromContent(string(content)); c != "" {
		return strings.ToLower(c)
	}

	if c := getEncodingFromHeaders(headers); c != "" {
		c = strings.ToLower(c)
		if c == "iso-8859-1" && !strings.Contains(strings.ToLower(headers.Get("Content-Type")), "iso-8859-1") {
			_, contentType, _ := charset.DetermineEncoding(content, "")
			return strings.ToLower(contentType)

		} else {
			return c
		}
	}

	return ""
}

/**
 * 编码转换
 * 需要传入原始编码和输出编码，如果原始编码传入出错，则转换出来的文本会乱码
 */
func EncodingConvert(src string, srcCode string, tagCode string) string {
	srcCoder := mahonia.NewDecoder(srcCode)
	srcResult := srcCoder.ConvertString(src)
	tagCoder := mahonia.NewDecoder(tagCode)
	_, cdata, _ := tagCoder.Translate([]byte(srcResult), true)
	result := string(cdata)
	return result
}

func EncodingConvertToUtf8(content string, contentType string) string {
	var encode string

	if strings.Contains(contentType, "gbk") || strings.Contains(contentType, "gb2312") || strings.Contains(contentType, "gb18030") || strings.Contains(contentType, "windows-1252") {
		encode = "gb18030"
	} else if strings.Contains(contentType, "big5") {
		encode = "big5"
	} else if strings.Contains(contentType, "utf-8") {
		encode = "utf-8"

	}

	if encode != "" && encode != "utf-8" {

		content = EncodingConvert(content, encode, "utf-8")
	}
	return content
}
