package api

import (
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
