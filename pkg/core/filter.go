package core

type Filter[T any] struct {
	Value  T
	IsZero bool
}
