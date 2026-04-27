package bath

import (
	"fmt"
	"math"

	"github.com/tonkeeper/opentonapi/pkg/pyth"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	abiPythOracle "github.com/tonkeeper/tongo/abi-tolk/abiGenerated/pythOracle"
)

type BubbleOraclePriceUpdate struct {
	Requester  tongo.AccountID
	Oracle     tongo.AccountID
	ResponseTo tongo.AccountID
	Success    bool
	PriceFeeds []OraclePriceFeedInfo
}

func (b BubbleOraclePriceUpdate) ToAction() (action *Action) {
	return &Action{
		Success: b.Success,
		Type:    OracleRequest,
		OracleRequest: &OracleRequestAction{
			Requester:  b.Requester,
			Oracle:     b.Oracle,
			ResponseTo: b.ResponseTo,
			PriceFeeds: b.PriceFeeds,
		},
	}
}

type PriceFeedMetaStore interface {
	GetPythPriceFeedMeta(id string) (pyth.PriceFeedAttributes, bool)
}

func PythOraclePriceUpdateStraw(priceFeedMetaStore PriceFeedMetaStore) Straw[BubbleOraclePriceUpdate] {
	return Straw[BubbleOraclePriceUpdate]{
		CheckFuncs: []bubbleCheck{
			IsTx,
			HasOperation(abiPythOracle.PythOracleParsePriceFeedUpdatesMessageMsgOp),
			HasInterface(abi.PythOracle),
		},
		Builder: func(newAction *BubbleOraclePriceUpdate, bubble *Bubble) error {
			tx := bubble.Info.(BubbleTx)
			request, ok := tx.decodedBody.Value.(*abiPythOracle.ParsePriceFeedUpdatesMessage)
			if !ok {
				return fmt.Errorf("unexpected body type: %T, for %v", tx.decodedBody.Value, abiPythOracle.PythOracleParsePriceFeedUpdatesMessageMsgOp)
			}
			newAction.Requester = tx.inputFrom.Address
			newAction.Oracle = tx.account.Address
			targetAddr, err := tongo.AccountIDFromTlb(request.TargetAddress)
			if err != nil || targetAddr == nil {
				return fmt.Errorf("invalid target address: %v", err)
			}
			newAction.ResponseTo = *targetAddr
			if request.PriceIds.Value != nil {
				for _, id := range *request.PriceIds.Value {
					hexID := id.HexString()
					var attrs pyth.PriceFeedAttributes
					if priceFeedMetaStore != nil {
						attrs, _ = priceFeedMetaStore.GetPythPriceFeedMeta(hexID)
					}
					newAction.PriceFeeds = append(newAction.PriceFeeds, OraclePriceFeedInfo{
						ID:            hexID,
						DisplaySymbol: attrs.DisplaySymbol,
					})
				}
			}
			return nil
		},
		Children: []Straw[BubbleOraclePriceUpdate]{
			{
				CheckFuncs: []bubbleCheck{
					IsTx, Or(
						HasOperation(abiPythOracle.PythOracleOracleResponseSuccessMsgOp),
						HasOperation(abiPythOracle.PythOracleErrorResponseMsgOp),
					),
				},
				Builder: func(newAction *BubbleOraclePriceUpdate, bubble *Bubble) error {
					if !IsAccount(newAction.ResponseTo)(bubble) {
						return fmt.Errorf("oracle response for wrong requester")
					}
					isSuccess := HasOperation(abiPythOracle.PythOracleOracleResponseSuccessMsgOp)(bubble)
					newAction.Success = isSuccess
					if !isSuccess {
						return nil
					}
					tx := bubble.Info.(BubbleTx)
					response, ok := tx.decodedBody.Value.(*abiPythOracle.OracleResponseSuccess)
					if !ok || response == nil {
						return nil
					}
					if response.InitialSender != newAction.Requester.ToInternal() {
						return fmt.Errorf("oracle response for wrong requester")
					}
					if response.PriceFeeds.Value == nil {
						return nil
					}
					for _, entry := range *response.PriceFeeds.Value {
						if entry.PriceFeed.Value == nil || entry.PriceFeed.Value.Price.Value == nil {
							continue
						}
						assetID := entry.PriceId.HexString()
						pp := entry.PriceFeed.Value.Price.Value
						rate := float64(pp.Price) * math.Pow(10, float64(pp.Expo))
						for i := range newAction.PriceFeeds {
							if newAction.PriceFeeds[i].ID == assetID {
								newAction.PriceFeeds[i].Rate = &rate
								break
							}
						}
					}
					return nil
				},
			},
		},
	}
}
