// Code generated by ogen, DO NOT EDIT.

package oas

import (
	"fmt"

	"github.com/go-faster/errors"

	"github.com/ogen-go/ogen/validate"
)

func (s AccStatusChange) Validate() error {
	switch s {
	case "acst_unchanged":
		return nil
	case "acst_frozen":
		return nil
	case "acst_deleted":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}
func (s AccountEvent) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Actions == nil {
			return errors.New("nil is invalid value")
		}
		var failures []validate.FieldError
		for i, elem := range s.Actions {
			if err := func() error {
				if err := elem.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				failures = append(failures, validate.FieldError{
					Name:  fmt.Sprintf("[%d]", i),
					Error: err,
				})
			}
		}
		if len(failures) > 0 {
			return &validate.Error{Fields: failures}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "actions",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s AccountEvents) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Events == nil {
			return errors.New("nil is invalid value")
		}
		var failures []validate.FieldError
		for i, elem := range s.Events {
			if err := func() error {
				if err := elem.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				failures = append(failures, validate.FieldError{
					Name:  fmt.Sprintf("[%d]", i),
					Error: err,
				})
			}
		}
		if len(failures) > 0 {
			return &validate.Error{Fields: failures}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "events",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s AccountStacking) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Pools == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "pools",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s AccountStatus) Validate() error {
	switch s {
	case "nonexist":
		return nil
	case "uninit":
		return nil
	case "active":
		return nil
	case "frozen":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}
func (s Action) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if err := s.Type.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "type",
			Error: err,
		})
	}
	if err := func() error {
		if err := s.Status.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "status",
			Error: err,
		})
	}
	if err := func() error {
		if s.TonTransfer.Set {
			if err := func() error {
				if err := s.TonTransfer.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "TonTransfer",
			Error: err,
		})
	}
	if err := func() error {
		if s.ContractDeploy.Set {
			if err := func() error {
				if err := s.ContractDeploy.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "ContractDeploy",
			Error: err,
		})
	}
	if err := func() error {
		if s.JettonTransfer.Set {
			if err := func() error {
				if err := s.JettonTransfer.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "JettonTransfer",
			Error: err,
		})
	}
	if err := func() error {
		if s.NftItemTransfer.Set {
			if err := func() error {
				if err := s.NftItemTransfer.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "NftItemTransfer",
			Error: err,
		})
	}
	if err := func() error {
		if s.AuctionBid.Set {
			if err := func() error {
				if err := s.AuctionBid.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "AuctionBid",
			Error: err,
		})
	}
	if err := func() error {
		if s.NftPurchase.Set {
			if err := func() error {
				if err := s.NftPurchase.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "NftPurchase",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s ActionStatus) Validate() error {
	switch s {
	case "ok":
		return nil
	case "failed":
		return nil
	case "pending":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}
func (s ActionType) Validate() error {
	switch s {
	case "TonTransfer":
		return nil
	case "JettonTransfer":
		return nil
	case "NftItemTransfer":
		return nil
	case "ContractDeploy":
		return nil
	case "Subscribe":
		return nil
	case "UnSubscribe":
		return nil
	case "AuctionBid":
		return nil
	case "NftPurchase":
		return nil
	case "Unknown":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}
func (s AuctionBidAction) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if err := s.AuctionType.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "auction_type",
			Error: err,
		})
	}
	if err := func() error {
		if s.Nft.Set {
			if err := func() error {
				if err := s.Nft.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "nft",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s AuctionBidActionAuctionType) Validate() error {
	switch s {
	case "DNS.ton":
		return nil
	case "DNS.tg":
		return nil
	case "NUMBER.tg":
		return nil
	case "getgems":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}
func (s Auctions) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Data == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "data",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s Block) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.PrevRefs == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "prev_refs",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s BouncePhaseType) Validate() error {
	switch s {
	case "TrPhaseBounceNegfunds":
		return nil
	case "TrPhaseBounceNofunds":
		return nil
	case "TrPhaseBounceOk":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}
func (s ComputePhase) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.SkipReason.Set {
			if err := func() error {
				if err := s.SkipReason.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "skip_reason",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s ComputeSkipReason) Validate() error {
	switch s {
	case "cskip_no_state":
		return nil
	case "cskip_bad_state":
		return nil
	case "cskip_no_gas":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}
func (s ContractDeployAction) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Interfaces == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "interfaces",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s DnsRecord) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Wallet.Set {
			if err := func() error {
				if err := s.Wallet.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "wallet",
			Error: err,
		})
	}
	if err := func() error {
		if s.Site == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "site",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s DomainBids) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Data == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "data",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s DomainNames) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Domains == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "domains",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s Event) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Actions == nil {
			return errors.New("nil is invalid value")
		}
		var failures []validate.FieldError
		for i, elem := range s.Actions {
			if err := func() error {
				if err := elem.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				failures = append(failures, validate.FieldError{
					Name:  fmt.Sprintf("[%d]", i),
					Error: err,
				})
			}
		}
		if len(failures) > 0 {
			return &validate.Error{Fields: failures}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "actions",
			Error: err,
		})
	}
	if err := func() error {
		if s.Fees == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "fees",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s Jetton) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Verification.Set {
			if err := func() error {
				if err := s.Verification.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "verification",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s JettonBalance) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if err := s.Verification.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "verification",
			Error: err,
		})
	}
	if err := func() error {
		if s.Metadata.Set {
			if err := func() error {
				if err := s.Metadata.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "metadata",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s JettonInfo) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if err := s.Verification.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "verification",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s JettonTransferAction) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Refund.Set {
			if err := func() error {
				if err := s.Refund.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "refund",
			Error: err,
		})
	}
	if err := func() error {
		if err := s.Jetton.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "jetton",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s JettonVerificationType) Validate() error {
	switch s {
	case "whitelist":
		return nil
	case "blacklist":
		return nil
	case "none":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}
func (s JettonsBalances) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Balances == nil {
			return errors.New("nil is invalid value")
		}
		var failures []validate.FieldError
		for i, elem := range s.Balances {
			if err := func() error {
				if err := elem.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				failures = append(failures, validate.FieldError{
					Name:  fmt.Sprintf("[%d]", i),
					Error: err,
				})
			}
		}
		if len(failures) > 0 {
			return &validate.Error{Fields: failures}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "balances",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s NftCollections) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.NftCollections == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "nft_collections",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s NftItem) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.ApprovedBy == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "approved_by",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s NftItemTransferAction) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Refund.Set {
			if err := func() error {
				if err := s.Refund.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "refund",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s NftItems) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.NftItems == nil {
			return errors.New("nil is invalid value")
		}
		var failures []validate.FieldError
		for i, elem := range s.NftItems {
			if err := func() error {
				if err := elem.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				failures = append(failures, validate.FieldError{
					Name:  fmt.Sprintf("[%d]", i),
					Error: err,
				})
			}
		}
		if len(failures) > 0 {
			return &validate.Error{Fields: failures}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "nft_items",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s NftPurchaseAction) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.PurchaseType.Set {
			if err := func() error {
				if err := s.PurchaseType.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "purchase_type",
			Error: err,
		})
	}
	if err := func() error {
		if err := s.Nft.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "nft",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s NftPurchaseActionPurchaseType) Validate() error {
	switch s {
	case "DNS.tg":
		return nil
	case "getgems":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}

func (s PoolInfo) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if err := s.Implementation.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "implementation",
			Error: err,
		})
	}
	if err := func() error {
		if err := (validate.Float{}).Validate(float64(s.Apy)); err != nil {
			return errors.Wrap(err, "float")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "apy",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s PoolInfoImplementation) Validate() error {
	switch s {
	case "whales":
		return nil
	case "tf":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}
func (s Refund) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if err := s.Type.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "type",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s RefundType) Validate() error {
	switch s {
	case "DNS.ton":
		return nil
	case "DNS.tg":
		return nil
	case "GetGems":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}
func (s StackingPoolsOK) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Pools == nil {
			return errors.New("nil is invalid value")
		}
		var failures []validate.FieldError
		for i, elem := range s.Pools {
			if err := func() error {
				if err := elem.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				failures = append(failures, validate.FieldError{
					Name:  fmt.Sprintf("[%d]", i),
					Error: err,
				})
			}
		}
		if len(failures) > 0 {
			return &validate.Error{Fields: failures}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "pools",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s StoragePhase) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if err := s.StatusChange.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "status_change",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s Subscriptions) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Subscriptions == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "subscriptions",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s TonTransferAction) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Refund.Set {
			if err := func() error {
				if err := s.Refund.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "refund",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s Trace) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if err := s.Transaction.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "transaction",
			Error: err,
		})
	}
	if err := func() error {
		var failures []validate.FieldError
		for i, elem := range s.Children {
			if err := func() error {
				if err := elem.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				failures = append(failures, validate.FieldError{
					Name:  fmt.Sprintf("[%d]", i),
					Error: err,
				})
			}
		}
		if len(failures) > 0 {
			return &validate.Error{Fields: failures}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "children",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s TraceIds) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Traces == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "traces",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s Transaction) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if err := s.OrigStatus.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "orig_status",
			Error: err,
		})
	}
	if err := func() error {
		if err := s.EndStatus.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "end_status",
			Error: err,
		})
	}
	if err := func() error {
		if err := s.TransactionType.Validate(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "transaction_type",
			Error: err,
		})
	}
	if err := func() error {
		if s.OutMsgs == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "out_msgs",
			Error: err,
		})
	}
	if err := func() error {
		if s.ComputePhase.Set {
			if err := func() error {
				if err := s.ComputePhase.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "compute_phase",
			Error: err,
		})
	}
	if err := func() error {
		if s.StoragePhase.Set {
			if err := func() error {
				if err := s.StoragePhase.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "storage_phase",
			Error: err,
		})
	}
	if err := func() error {
		if s.BouncePhase.Set {
			if err := func() error {
				if err := s.BouncePhase.Value.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "bounce_phase",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s TransactionType) Validate() error {
	switch s {
	case "TransOrd":
		return nil
	case "TransTickTock":
		return nil
	case "TransSplitPrepare":
		return nil
	case "TransSplitInstall":
		return nil
	case "TransMergePrepare":
		return nil
	case "TransMergeInstall":
		return nil
	case "TransStorage":
		return nil
	default:
		return errors.Errorf("invalid value: %v", s)
	}
}
func (s Transactions) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Transactions == nil {
			return errors.New("nil is invalid value")
		}
		var failures []validate.FieldError
		for i, elem := range s.Transactions {
			if err := func() error {
				if err := elem.Validate(); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				failures = append(failures, validate.FieldError{
					Name:  fmt.Sprintf("[%d]", i),
					Error: err,
				})
			}
		}
		if len(failures) > 0 {
			return &validate.Error{Fields: failures}
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "transactions",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
func (s WalletDNS) Validate() error {
	var failures []validate.FieldError
	if err := func() error {
		if s.Names == nil {
			return errors.New("nil is invalid value")
		}
		return nil
	}(); err != nil {
		failures = append(failures, validate.FieldError{
			Name:  "names",
			Error: err,
		})
	}
	if len(failures) > 0 {
		return &validate.Error{Fields: failures}
	}
	return nil
}
