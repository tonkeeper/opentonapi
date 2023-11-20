package api

import (
	"fmt"

	"github.com/tonkeeper/tongo/boc"
)

// deserializeBoc tries to deserialize boc string in base64 or hex format.
func deserializeBoc(bocStr string) ([]*boc.Cell, error) {
	cells, err := boc.DeserializeBocBase64(bocStr)
	if err != nil {
		return boc.DeserializeBocHex(bocStr)
	}
	return cells, nil
}

func deserializeSingleBoc(bocStr string) (*boc.Cell, error) {
	cells, err := deserializeBoc(bocStr)
	if err != nil {
		return nil, err
	}
	if len(cells) != 1 {
		return nil, fmt.Errorf("invalid boc roots number %v", len(cells))
	}
	return cells[0], nil
}
