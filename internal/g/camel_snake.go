package g

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
)

func CamelToSnake(s string) string {
	b := new(strings.Builder)
	b.Grow(len(s) + 5)
	for i, c := range s {
		if ('a' <= c && c <= 'z') || ('0' <= c && c <= '9') || c == '_' { //todo: not skip lower case but replase upper case
			b.WriteRune(c)
			continue
		}
		if i != 0 {
			b.WriteRune('_')
		}
		b.WriteRune(c + 'a' - 'A')
	}
	return b.String()
}

func ChangeJsonKeys(input []byte, f func(s string) string) []byte {

	buf := bytes.NewBuffer(make([]byte, 0, len(input)+8))
	isMap := false
	nextTokenIsKey := false
	var delimiters int64
	d := json.NewDecoder(bytes.NewReader(input))
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return input
		}
		switch v := token.(type) {
		case json.Delim:
			buf.WriteRune(rune(v))
			if v == '{' || v == '[' {
				delimiters = delimiters << 1
				if v == '{' {
					delimiters++
					isMap = true
					nextTokenIsKey = true
				} else {
					isMap = false
				}
				continue
			}
			delimiters = delimiters >> 1
			if delimiters&1 == 1 {
				isMap = true
				nextTokenIsKey = false
			} else {
				isMap = false
			}
		case bool:
			buf.WriteString(strconv.FormatBool(v))
		case float64:
			buf.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
		case json.Number:
			buf.WriteString(v.String())
		case string:
			if isMap && nextTokenIsKey {
				v = f(v)
			}
			buf.WriteRune('"')
			buf.WriteString(v)
			buf.WriteRune('"')
		case nil:
			buf.WriteString("null")
		}
		if isMap && nextTokenIsKey {
			buf.WriteRune(':')
			nextTokenIsKey = false
			continue
		}
		nextTokenIsKey = !nextTokenIsKey
		if d.More() {
			buf.WriteRune(',')
		}
	}
	return buf.Bytes()
}
