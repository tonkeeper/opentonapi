package core

type Trace struct {
	Transaction
	Children []*Trace
}

func (t *Trace) InProgress() bool {
	return t.countUncompleted() != 0
}
func (t *Trace) countUncompleted() int {
	c := len(t.OutMsgs) //todo: not count externals
	for _, st := range t.Children {
		c += st.countUncompleted()
	}
	return c
}
