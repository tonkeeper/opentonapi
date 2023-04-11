package api

import (
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func convertNFT(item core.NftItem, book addressBook, previewGen previewGenerator) oas.NftItem {
	i := oas.NftItem{
		Address:  item.Address.ToRaw(),
		Index:    item.Index.BigInt().Int64(),
		Owner:    convertOptAccountAddress(item.OwnerAddress, book),
		Verified: item.Verified,
		Metadata: anyToJSONRawMap(item.Metadata, false),
		DNS:      pointerToOptString(item.DNS),
	}
	if item.Sale != nil {
		i.SetSale(oas.OptSale{
			Value: oas.Sale{
				Address: item.Sale.Contract.ToRaw(),
				Market:  convertAccountAddress(item.Sale.Marketplace, book),
				Owner:   convertOptAccountAddress(item.Sale.Seller, book),
				Price: oas.Price{
					Value:     fmt.Sprintf("%v", item.Sale.Price.Amount),
					TokenName: "TON", //todo: support over token
				},
			},
			Set: true,
		})
	}
	if item.CollectionAddress != nil {
		if _, prs := book.GetCollectionInfoByAddress(*item.CollectionAddress); prs {
			i.ApprovedBy = []string{"tonkeeper"} //todo: make enum
		}
		//todo: add coolection info
	}
	if item.Metadata != nil {
		if imageI, prs := item.Metadata["image"]; prs {
			if image, ok := imageI.(string); ok {
				for _, size := range []int{100, 500, 1500} {
					url := previewGen.GenerateImageUrl(image, size, size)
					i.Previews = append(i.Previews, oas.ImagePreview{
						Resolution: fmt.Sprintf("%vx%v", size, size),
						URL:        url,
					})
				}
			}
		}
	}
	return i
}

func convertNftCollection(collection core.NftCollection, book addressBook) oas.NftCollection {
	c := oas.NftCollection{
		Address:              collection.Address.ToRaw(),
		NextItemIndex:        int64(collection.NextItemIndex),
		RawCollectionContent: fmt.Sprintf("%x", collection.CollectionContent[:]),
		Owner:                convertOptAccountAddress(collection.OwnerAddress, book),
	}
	return c
}
