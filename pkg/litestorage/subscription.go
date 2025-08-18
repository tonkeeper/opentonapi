package litestorage

import (
	"context"
	"errors"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
)

func (s *LiteStorage) SubscriptionInfos(ctx context.Context, ids []core.SubscriptionID) (map[tongo.AccountID]core.SubscriptionInfo, error) {
	res := make(map[tongo.AccountID]core.SubscriptionInfo)
	for _, id := range ids {
		switch id.Interface {
		case abi.SubscriptionV1:
			_, value, err := abi.GetSubscriptionData(ctx, s.executor, id.Account)
			if err != nil {
				return nil, err
			}
			data, ok := value.(abi.GetSubscriptionDataResult)
			if !ok {
				return nil, errors.New("invalid type of subscription data")
			}
			beneficiary := tongo.AccountID{
				Workchain: int32(data.Beneficiary.Workchain),
				Address:   data.Beneficiary.Address,
			}
			res[id.Account] = core.SubscriptionInfo{
				Wallet: tongo.AccountID{
					Workchain: int32(data.Wallet.Workchain),
					Address:   data.Wallet.Address,
				},
				Admin:            beneficiary,
				WithdrawTo:       beneficiary,
				PaymentPerPeriod: int64(data.Amount),
			}
		case abi.SubscriptionV2:
			_, sValue, err := abi.GetSubscriptionInfo(ctx, s.executor, id.Account)
			if err != nil {
				return nil, err
			}
			subInfo, ok := sValue.(abi.GetSubscriptionInfo_V2Result)
			if !ok {
				return nil, errors.New("invalid type of subscription data")
			}
			_, pValue, err := abi.GetPaymentInfo(ctx, s.executor, id.Account)
			if err != nil {
				return nil, err
			}
			paymentInfo, ok := pValue.(abi.GetPaymentInfo_SubscriptionV2Result)
			if !ok {
				return nil, errors.New("invalid type of subscription data")
			}
			wallet, err := tongo.AccountIDFromTlb(subInfo.Wallet)
			if err != nil {
				return nil, err
			}
			if wallet == nil {
				return nil, errors.New("invalid wallet address")
			}
			admin, err := tongo.AccountIDFromTlb(subInfo.Admin)
			if err != nil {
				return nil, err
			}
			if admin == nil {
				return nil, errors.New("invalid beneficiary address")
			}
			withdrawTo, err := tongo.AccountIDFromTlb(subInfo.WithdrawAddress)
			if err != nil {
				return nil, err
			}
			if withdrawTo == nil {
				return nil, errors.New("invalid withdraw address")
			}
			res[id.Account] = core.SubscriptionInfo{
				Wallet:           *wallet,
				Admin:            *admin,
				WithdrawTo:       *withdrawTo,
				PaymentPerPeriod: int64(paymentInfo.PaymentPerPeriod),
			}
		default:
			continue
		}
	}
	return res, nil
}
