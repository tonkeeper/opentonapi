package g

import "fmt"

func ToStrings[T fmt.Stringer](ts []T) []string {
	result := make([]string, 0, len(ts))
	for _, t := range ts {
		result = append(result, t.String())
	}
	return result
}

func FromStrings[T ~string](strings []string) []T {
	result := make([]T, 0, len(strings))
	for _, s := range strings {
		result = append(result, T(s))
	}
	return result
}
