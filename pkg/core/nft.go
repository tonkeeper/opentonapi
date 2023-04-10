package core

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/opentonapi/pkg/config"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"io"
	"net/http"
	"strings"
	"time"
)

type ContentLayout int

const (
	Undefined ContentLayout = iota
	OffChain
	OnChain
	SemiChain
)

type NftItem struct {
	Address           tongo.AccountID
	Index             decimal.Decimal
	CollectionAddress *tongo.AccountID
	OwnerAddress      *tongo.AccountID
	Verified          bool
	Transferable      bool
	DNS               *string
	Sale              *NftSaleInfo
	Metadata          map[string]interface{}
}

type NftCollection struct {
	Address           tongo.AccountID
	NextItemIndex     uint64
	OwnerAddress      *tongo.AccountID
	ContentLayout     int
	CollectionContent []byte
	Metadata          []byte
}

type NftSaleInfo struct {
	Contract    tongo.AccountID
	Marketplace tongo.AccountID
	Nft         tongo.AccountID
	Seller      *tongo.AccountID
	Price       struct {
		Token  *tongo.AccountID
		Amount uint64
	}
	MarketplaceFee uint64
	RoyaltyAddress *tongo.AccountID
	RoyaltyAmount  uint64
}

type FullContent struct {
	Layout ContentLayout
	Data   []byte
}

type Metadata struct {
	Uri         string `json:"uri,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	ImageData   []byte `json:"image_data,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
	Decimals    string `json:"decimals,omitempty"`
}

func convertOnСhainData(content tlb.FullContent) (Metadata, error) {
	if content.SumType != "Onchain" {
		return Metadata{}, fmt.Errorf("not Onchain content")
	}
	var m Metadata
	for i, v := range content.Onchain.Data.Values() {
		key := content.Onchain.Data.Keys()[i]
		keyS := hex.EncodeToString(key[:])
		switch keyS {
		case "70e5d7b6a29b392f85076fe15ca2f2053c56c2338728c4e33c9e8ddb1ee827cc": // sha256(uri)
			b, err := v.Value.Bytes()
			if err != nil {
				return Metadata{}, err
			}
			m.Uri = string(b)
		case "82a3537ff0dbce7eec35d69edc3a189ee6f17d82f353a553f9aa96cb0be3ce89": // sha256(name)
			b, err := v.Value.Bytes()
			if err != nil {
				return Metadata{}, err
			}
			m.Name = string(b)
		case "c9046f7a37ad0ea7cee73355984fa5428982f8b37c8f7bcec91f7ac71a7cd104": // sha256(description)
			b, err := v.Value.Bytes()
			if err != nil {
				return Metadata{}, err
			}
			m.Description = string(b)
		case "6105d6cc76af400325e94d588ce511be5bfdbb73b437dc51eca43917d7a43e3d": // sha256(image)
			b, err := v.Value.Bytes()
			if err != nil {
				return Metadata{}, err
			}
			m.Image = string(b)
		case "d9a88ccec79eef59c84b671136a20ece4cd00caaad5bc47e2c208829154ee9e4": // sha256(image_data)
			b, err := v.Value.Bytes()
			if err != nil {
				return Metadata{}, err
			}
			m.ImageData = b
		case "b76a7ca153c24671658335bbd08946350ffc621fa1c516e7123095d4ffd5c581": // sha256(symbol)
			b, err := v.Value.Bytes()
			if err != nil {
				return Metadata{}, err
			}
			m.Symbol = string(b)
		case "ee80fd2f1e03480e2282363596ee752d7bb27f50776b95086a0279189675923e": // sha256(decimals)
			b, err := v.Value.Bytes()
			if err != nil {
				return Metadata{}, err
			}
			m.Decimals = string(b)
		}
	}
	return m, nil
}

func DecodeFullContent(cell *boc.Cell) (FullContent, error) {
	var (
		content tlb.FullContent
		layout  ContentLayout
		result  []byte
	)
	err := tlb.Unmarshal(cell, &content)
	if err != nil {
		return FullContent{}, fmt.Errorf("%v content decoding: %v", content.SumType, err)
	}
	if content.SumType == "Onchain" {
		layout = OnChain
		meta, err := convertOnСhainData(content)
		if err != nil {
			return FullContent{}, err
		}
		result, err = json.Marshal(meta)
		if err != nil {
			return FullContent{}, err
		}
	} else {
		layout = OffChain
		bs := boc.BitString(content.Offchain.Uri)
		if bs.BitsAvailableForRead()%8 != 0 {
			return FullContent{}, fmt.Errorf("text data is not multiple of 8 bits")
		}
		result, err = bs.GetTopUppedArray()
		if err != nil {
			return FullContent{}, err
		}
	}
	return FullContent{layout, result}, nil
}

func GetNftMetaData(nftMetaUrl string) ([]byte, error) {
	if strings.HasPrefix(nftMetaUrl, "ipfs://") {
		nftMetaUrl = config.Loader.IpfsGate + nftMetaUrl[7:]
	}
	if strings.HasPrefix(nftMetaUrl, "tonstorage://") {
		nftMetaUrl = config.Loader.TonStorageGate + nftMetaUrl[13:]
	}
	if !strings.HasPrefix(nftMetaUrl, "http://") &&
		!strings.HasPrefix(nftMetaUrl, "https://") {
		return nil, fmt.Errorf("not http/https link")
	}
	var client = &http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(nftMetaUrl)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("invalid status code: %v", response.StatusCode)
	}
	meta, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return meta, nil
}
