package req

import (
	"net/http"
	"regexp"
	"strings"
)

var (
	charsetRe = regexp.MustCompile(`(?i)<meta.*?charset=["\']*(.+?)["\'>]`)
	pragmaRe  = regexp.MustCompile(`(?i)<meta.*?content=["\']*;?charset=(.+?)["\'>]`)
	xmlRe     = regexp.MustCompile(`(?i)^<\?XML.*?encoding=["\']*(.+?)["\'>]`)
	//fmt.Sprintf("^(?%s:%s", "i", pattern[1:])
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
	if charset := getEncodingFromHeaders(headers); charset != "" {
		return charset
	}

	if charset := getEncodingFromContent(string(content)); charset != "" {
		return charset
	}

	return ""
}
