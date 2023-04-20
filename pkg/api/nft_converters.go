package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-faster/jx"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func convertNFT(ctx context.Context, item core.NftItem, book addressBook, previewGen previewGenerator, metaCache metadataCache) oas.NftItem {
	i := oas.NftItem{
		Address:  item.Address.ToRaw(),
		Index:    item.Index.BigInt().Int64(),
		Owner:    convertOptAccountAddress(item.OwnerAddress, book),
		Verified: item.Verified,
		Metadata: anyToJSONRawMap(item.Metadata, false),
		DNS:      pointerToOptString(item.DNS),
	}
	if item.Sale != nil {
		tokenName := "TON"
		if item.Sale.Price.Token != nil {
			m, _ := metaCache.getJettonMeta(ctx, *item.Sale.Price.Token)
			tokenName = m.Name
			if tokenName == "" {
				tokenName = "UKWN"
			}
		}
		i.SetSale(oas.OptSale{
			Value: oas.Sale{
				Address: item.Sale.Contract.ToRaw(),
				Market:  convertAccountAddress(item.Sale.Marketplace, book),
				Owner:   convertOptAccountAddress(item.Sale.Seller, book),
				Price: oas.Price{
					Value:     fmt.Sprintf("%v", item.Sale.Price.Amount),
					TokenName: tokenName,
				},
			},
			Set: true,
		})
	}
	if item.CollectionAddress != nil {
		if _, prs := book.GetCollectionInfoByAddress(*item.CollectionAddress); prs {
			i.ApprovedBy = []string{"tonkeeper"} //todo: make enum
		}
		i.Collection.SetTo(oas.NftItemCollection{
			Address: item.CollectionAddress.ToRaw(),
			Name:    "", //todo: use cache
		})
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
	if len(collection.Metadata) != 0 {
		metadata := map[string]jx.Raw{}
		for k, v := range collection.Metadata {
			var err error
			metadata[k], err = json.Marshal(v)
			if err != nil {
				continue
			}
		}
		c.Metadata.SetTo(metadata)
	}
	return c
}
