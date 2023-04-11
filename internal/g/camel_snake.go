package g

import "strings"

func CamelToSnake(s string) string {
	b := new(strings.Builder)
	b.Grow(len(s) + 5)
	for i, c := range s {
		if ('a' <= c && c <= 'z') || ('0' <= c && c <= '9') {
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
