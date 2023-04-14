package g

func ToStrings[T ~string](ts []T) []string {
	result := make([]string, 0, len(ts))
	for _, t := range ts {
		result = append(result, string(t))
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
