package indexer

import "strings"

func isBlockNotReadyError(err error) bool {
	if strings.Contains(err.Error(), "ltdb: block not found") {
		return true
	}
	if strings.Contains(err.Error(), "block is not applied") {
		return true
	}
	return false
}
