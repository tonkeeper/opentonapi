package api

type Limits struct {
	// BulkLimits stands for a number of entities a user is allowed to request at once with a bulk query.
	BulkLimits int
}

func (lim *Limits) isBulkQuantityAllowed(quantity int) bool {
	if lim.BulkLimits <= 0 {
		return true
	}
	return quantity <= lim.BulkLimits
}
