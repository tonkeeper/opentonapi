package g

// Pointer returns pointer to copy of object
func Pointer[T any](o T) *T {
	return &o
}

// NilToNilError receive function adopts function for converting any types to works with pointers
func NilToNilError[I any, O any](f func(i I) (O, error), i *I) (*O, error) {
	if i == nil {
		return nil, nil
	}
	o, err := f(*i)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// NilToNil receive function adopts function for converting any types to works with pointers
func NilToNil[I any, O any](f func(i I) O, i *I) *O {
	if i == nil {
		return nil
	}
	return Pointer(f(*i))
}
