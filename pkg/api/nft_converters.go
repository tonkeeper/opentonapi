package api

import (
	"encoding/json"
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func convertNFT(item core.NftItem) oas.NftItem {
	return oas.NftItem{
		Address:    item.Address.ToRaw(),
		Index:      item.Index.BigInt().Int64(),
		Owner:      convertOptAccountAddress(item.OwnerAddress),
		Collection: oas.OptNftItemCollection{}, //todo: add
		Verified:   item.Verified,
		Metadata:   anyToJSONRawMap(item.Metadata),
		Sale:       oas.OptSale{}, //todo: add
		Previews:   nil,           //todo: add
		DNS:        pointerToOptString(item.DNS),
		ApprovedBy: nil, //todo: add
	}
}

func convertNftCollection(collection core.NftCollection) oas.NftCollection {
	c := oas.NftCollection{
		Address:              collection.Address.ToRaw(),
		NextItemIndex:        int64(collection.NextItemIndex),
		RawCollectionContent: fmt.Sprintf("%x", collection.CollectionContent[:]),
		Owner:                convertOptAccountAddress(collection.OwnerAddress),
	}
	if collection.Metadata != nil {
		var metadata oas.OptNftCollectionMetadata
		err := json.Unmarshal(collection.Metadata, &metadata)
		if err == nil {
			c.Metadata = metadata
		}
	}
	return c
}
