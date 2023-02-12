package core

type Trace struct {
	Transaction
	Children []*Trace
}
