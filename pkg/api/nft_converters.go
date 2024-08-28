package api

import (
	"context"
	"encoding/json"
	"errors"
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

func (h *Handler) convertNFT(ctx context.Context, item core.NftItem, book addressBook, metaCache metadataCache) oas.NftItem {
	nftItem := oas.NftItem{
		Address:  item.Address.ToRaw(),
		Index:    item.Index.BigInt().Int64(),
		Owner:    convertOptAccountAddress(item.OwnerAddress, book),
		Verified: item.Verified,
		Metadata: anyToJSONRawMap(item.Metadata),
		DNS:      g.Opt(item.DNS),
	}
	if item.Sale != nil {
		tokenName := "TON"
		if item.Sale.Price.Token != nil {
			meta, _ := metaCache.getJettonMeta(ctx, *item.Sale.Price.Token)
			tokenName = meta.Name
			if tokenName == "" {
				tokenName = UnknownJettonName
			}
		}
		nftItem.SetSale(oas.NewOptSale(oas.Sale{
			Address: item.Sale.Contract.ToRaw(),
			Market:  convertAccountAddress(item.Sale.Marketplace, book),
			Owner:   convertOptAccountAddress(item.Sale.Seller, book),
			Price: oas.Price{
				Value:     fmt.Sprintf("%v", item.Sale.Price.Amount),
				TokenName: tokenName,
			},
		}))
	}
	var image, description string
	if item.CollectionAddress != nil {
		cInfo, _ := metaCache.getCollectionMeta(ctx, *item.CollectionAddress)
		if cc, prs := book.GetCollectionInfoByAddress(*item.CollectionAddress); prs {
			nftItem.ApprovedBy = append(nftItem.ApprovedBy, cc.Approvers...)
		}
		nftItem.Collection.SetTo(oas.NftItemCollection{
			Address:     item.CollectionAddress.ToRaw(),
			Name:        cInfo.Name,
			Description: cInfo.Description,
		})
		if *item.CollectionAddress == references.RootDotTon && item.DNS != nil && item.Verified {
			image = "https://cache.tonapi.io/dns/preview/" + *item.DNS + ".png"
			nftItem.Metadata["name"] = []byte(fmt.Sprintf(`"%v"`, *item.DNS))
			buttons, _ := json.Marshal([]map[string]string{{
				"label": "Manage",
				"uri":   fmt.Sprintf("https://dns.tonkeeper.com/manage?v=%v", item.Address.ToRaw())},
			})
			nftItem.Metadata["buttons"] = buttons
		}
	}
	if item.Metadata != nil {
		if imageI, prs := item.Metadata["image"]; prs {
			image, _ = imageI.(string)
		}
		if descriptionI, prs := item.Metadata["description"]; prs {
			description, _ = descriptionI.(string)
		}
	}
	if len(nftItem.ApprovedBy) > 0 && nftItem.Verified {
		nftItem.Trust = oas.TrustType(core.TrustWhitelist)
	} else {
		nftItem.Trust = oas.TrustType(h.spamFilter.NftTrust(item.Address, item.CollectionAddress, description, image))
	}
	if image == "" {
		image = references.Placeholder
	}
	for _, size := range []int{5, 100, 500, 1500} {
		url := imgGenerator.DefaultGenerator.GenerateImageUrl(image, size, size)
		nftItem.Previews = append(nftItem.Previews, oas.ImagePreview{
			Resolution: fmt.Sprintf("%vx%v", size, size),
			URL:        url,
		})
	}

	return nftItem
}

func convertNftCollection(collection core.NftCollection, book addressBook) oas.NftCollection {
	nftCollection := oas.NftCollection{
		Address:              collection.Address.ToRaw(),
		NextItemIndex:        int64(collection.NextItemIndex),
		RawCollectionContent: fmt.Sprintf("%x", collection.CollectionContent[:]),
		Owner:                convertOptAccountAddress(collection.OwnerAddress, book),
	}
	if len(collection.Metadata) == 0 {
		return nftCollection
	}
	metadata := make(map[string]jx.Raw)
	image := references.Placeholder
	for k, v := range collection.Metadata {
		if k == "image" {
			if img, ok := v.(string); ok && img != "" {
				image = img
			}
		}
		if raw, err := json.Marshal(v); err == nil {
			metadata[k] = raw
		}
	}
	if known, ok := book.GetCollectionInfoByAddress(collection.Address); ok {
		nftCollection.ApprovedBy = append(nftCollection.ApprovedBy, known.Approvers...)
	}
	nftCollection.Metadata.SetTo(metadata)
	for _, size := range []int{5, 100, 500, 1500} {
		url := imgGenerator.DefaultGenerator.GenerateImageUrl(image, size, size)
		nftCollection.Previews = append(nftCollection.Previews, oas.ImagePreview{
			Resolution: fmt.Sprintf("%vx%v", size, size),
			URL:        url,
		})
	}
	return nftCollection
}

func (h *Handler) convertNftHistory(ctx context.Context, account tongo.AccountID, traceIDs []tongo.Bits256, acceptLanguage oas.OptString) ([]oas.AccountEvent, int64, error) {
	var lastLT uint64
	events := make([]oas.AccountEvent, 0, len(traceIDs))
	for _, traceID := range traceIDs {
		trace, err := h.storage.GetTrace(ctx, traceID)
		if err != nil {
			if errors.Is(err, core.ErrTraceIsTooLong) {
				// we ignore this for now, because we believe that this case is extremely rare.
				continue
			}
			return nil, 0, err
		}
		actions, err := bath.FindActions(ctx, trace, bath.WithInformationSource(h.storage), bath.WithStraws(bath.NFTStraws))
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
		for _, action := range actions.Actions {
			if action.Type != bath.NftItemTransfer {
				continue
			}
			convertedAction, err := h.convertAction(ctx, &account, action, acceptLanguage)
			if err != nil {
				return nil, 0, err
			}
			event.Actions = append(event.Actions, convertedAction)
		}
		event.IsScam = h.spamFilter.CheckActions(event.Actions, &account)
		if len(event.Actions) > 0 {
			events = append(events, event)
			lastLT = trace.Lt
		}
	}

	return events, int64(lastLT), nil
}
