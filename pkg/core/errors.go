package core

import "errors"

var ErrEntityNotFound = errors.New("entity not found")
var ErrTooManyEntities = errors.New("too many entities")
