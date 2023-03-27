package core

type Trace struct {
	Transaction
	Children []*Trace
}

func (t *Trace) InProgress() bool {
	return t.countUncomplited() != 0
}
func (t *Trace) countUncomplited() int {
	c := len(t.OutMsgs) //todo: not count externals
	for _, st := range t.Children {
		c += st.countUncomplited()
	}
	return c
}
