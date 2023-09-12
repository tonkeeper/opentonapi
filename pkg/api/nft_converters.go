package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	imgGenerator "github.com/tonkeeper/opentonapi/pkg/image"
	"github.com/tonkeeper/tongo"

	"github.com/go-faster/jx"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
)

func convertNFT(ctx context.Context, item core.NftItem, book addressBook, metaCache metadataCache) oas.NftItem {
	i := oas.NftItem{
		Address:  item.Address.ToRaw(),
		Index:    item.Index.BigInt().Int64(),
		Owner:    convertOptAccountAddress(item.OwnerAddress, book),
		Verified: item.Verified,
		Metadata: anyToJSONRawMap(item.Metadata, false),
		DNS:      g.Opt(item.DNS),
	}
	if item.Sale != nil {
		tokenName := "TON"
		if item.Sale.Price.Token != nil {
			m, _ := metaCache.getJettonMeta(ctx, *item.Sale.Price.Token)
			tokenName = m.Name
			if tokenName == "" {
				tokenName = UnknownJettonName
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
	var image string
	if item.CollectionAddress != nil {
		if cc, prs := book.GetCollectionInfoByAddress(*item.CollectionAddress); prs {
			for _, a := range cc.Approvers {
				i.ApprovedBy = append(i.ApprovedBy, a)
			}
		}
		cInfo, _ := metaCache.getCollectionMeta(ctx, *item.CollectionAddress)
		i.Collection.SetTo(oas.NftItemCollection{
			Address:     item.CollectionAddress.ToRaw(),
			Name:        cInfo.Name,
			Description: cInfo.Description,
		})
		if *item.CollectionAddress == references.RootDotTon && item.DNS != nil && item.Verified {
			image = "https://cache.tonapi.io/dns/preview/" + *item.DNS + ".png"
			i.Metadata["name"] = []byte(fmt.Sprintf(`"%v"`, *item.DNS))
		}
	}

	if item.Metadata != nil {
		if imageI, prs := item.Metadata["image"]; prs {
			image, _ = imageI.(string)
		}
	}
	if image == "" {
		image = references.Placeholder
	}
	for _, size := range []int{5, 100, 500, 1500} {
		url := imgGenerator.DefaultGenerator.GenerateImageUrl(image, size, size)
		i.Previews = append(i.Previews, oas.ImagePreview{
			Resolution: fmt.Sprintf("%vx%v", size, size),
			URL:        url,
		})
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
	if len(collection.Metadata) == 0 {
		return c
	}
	metadata := map[string]jx.Raw{}
	image := references.Placeholder
	for k, v := range collection.Metadata {
		var err error
		if k == "image" {
			if i, ok := v.(string); ok && i != "" {
				image = i
			}
		}
		metadata[k], err = json.Marshal(v)
		if err != nil {
			continue
		}
	}
	c.Metadata.SetTo(metadata)
	for _, size := range []int{5, 100, 500, 1500} {
		url := imgGenerator.DefaultGenerator.GenerateImageUrl(image, size, size)
		c.Previews = append(c.Previews, oas.ImagePreview{
			Resolution: fmt.Sprintf("%vx%v", size, size),
			URL:        url,
		})
	}
	return c
}

func (h Handler) convertNftHistory(ctx context.Context, account tongo.AccountID, traceIDs []tongo.Bits256, acceptLanguage oas.OptString) ([]oas.AccountEvent, int64, error) {
	var lastLT uint64
	events := []oas.AccountEvent{}
	for _, traceID := range traceIDs {
		trace, err := h.storage.GetTrace(ctx, traceID)
		if err != nil {
			return nil, 0, err
		}
		result, err := bath.FindActions(ctx, trace, bath.WithInformationSource(h.storage), bath.WithStraws([]bath.StrawFunc{bath.FindNFTTransfer}))
		if err != nil {
			return nil, 0, err
		}
		event := oas.AccountEvent{
			EventID:    trace.Hash.Hex(),
			Account:    convertAccountAddress(account, h.addressBook),
			Timestamp:  trace.Utime,
			IsScam:     false,
			Lt:         int64(trace.Lt),
			InProgress: trace.InProgress(),
		}
		for _, action := range result.Actions {
			if action.Type != bath.NftItemTransfer {
				continue
			}
			convertedAction, spamDetected, err := h.convertAction(ctx, &account, action, acceptLanguage)
			if err != nil {
				return nil, 0, err
			}
			if !event.IsScam && spamDetected {
				event.IsScam = true
			}
			event.Actions = append(event.Actions, convertedAction)
		}
		if len(event.Actions) == 0 {
			continue
		}
		events = append(events, event)
		lastLT = trace.Lt
	}

	return events, int64(lastLT), nil
}
