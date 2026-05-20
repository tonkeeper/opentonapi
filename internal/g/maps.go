package g

func MapMapValues[K comparable, A any, B any](m map[K]A, f func(A) B) map[K]B {
	result := make(map[K]B, len(m))
	for k, v := range m {
		result[k] = f(v)
	}
	return result
}

func MustGet[K comparable, V any](m map[K]V, key K) V {
	value, ok := m[key]
	if !ok {
		panic("missing map key")
	}
	return value
}
