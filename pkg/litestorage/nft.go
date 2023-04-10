package litestorage

import (
	"context"
	"fmt"
	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"math/big"
)

func (s *LiteStorage) GetNFTs(ctx context.Context, accounts []tongo.AccountID) ([]core.NftItem, error) {
	return nil, nil
}

func (s *LiteStorage) SearchNFTs(ctx context.Context,
	collection *core.Filter[tongo.AccountID],
	owner *core.Filter[tongo.AccountID],
	includeOnSale bool,
	onlyVerified bool,
	limit, offset int,
) ([]tongo.AccountID, error) {
	return nil, nil
}

func (s *LiteStorage) GetNftCollections(ctx context.Context, limit, offset *int32) ([]core.NftCollection, error) {
	return nil, nil
}

func (s *LiteStorage) GetNftCollectionByCollectionAddress(ctx context.Context, address tongo.AccountID) (core.NftCollection, error) {
	_, value, err := abi.GetCollectionData(ctx, s.client, address)
	if err != nil {
		return core.NftCollection{}, nil
	}
	source, ok := value.(abi.GetCollectionDataResult)
	if !ok {
		return core.NftCollection{}, fmt.Errorf("invalid collection address")
	}
	content := boc.Cell(source.CollectionContent)
	fullContent, err := core.DecodeFullContent(&content)
	if err != nil {
		return core.NftCollection{}, err
	}
	accountID, err := tongo.AccountIDFromTlb(source.OwnerAddress)
	if err != nil {
		return core.NftCollection{}, err
	}
	collection := core.NftCollection{
		Address:           address,
		OwnerAddress:      accountID,
		CollectionContent: fullContent.Data,
		ContentLayout:     int(fullContent.Layout),
		NextItemIndex:     g.Pointer(big.Int(source.NextItemIndex)).Uint64(),
	}
	if fullContent.Layout == core.OffChain {
		meta, err := core.GetNftMetaData(string(fullContent.Data))
		if err != nil {
			return core.NftCollection{}, err
		}
		collection.Metadata = meta
	} else {
		collection.Metadata = fullContent.Data
	}
	return collection, nil
}
