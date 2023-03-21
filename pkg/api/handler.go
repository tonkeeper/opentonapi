package api

import (
	"context"
	"encoding/base64"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"github.com/go-faster/errors"
	"github.com/tonkeeper/opentonapi/pkg/i18n"
	"github.com/tonkeeper/tongo"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/opentonapi/pkg/blockchain"
	"github.com/tonkeeper/opentonapi/pkg/chainstate"
	"github.com/tonkeeper/opentonapi/pkg/config"
	"github.com/tonkeeper/opentonapi/pkg/i18n"

	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
)

// Compile-time check for Handler.
var _ oas.Handler = (*Handler)(nil)

type Handler struct {
	oas.UnimplementedHandler // automatically implement all methods

	addressBook      addressBook
	storage          storage
	state            chainState
	msgSender        messageSender
	previewGenerator previewGenerator
}

// Options configures behavior of a Handler instance.
type Options struct {
	storage          storage
	chainState       chainState
	addressBook      addressBook
	msgSender        messageSender
	previewGenerator previewGenerator
}

type Option func(o *Options)

func WithStorage(s storage) Option {
	return func(o *Options) {
		o.storage = s
	}
}
func WithChainState(state chainState) Option {
	return func(o *Options) {
		o.chainState = state
	}
}

func WithAddressBook(book addressBook) Option {
	return func(o *Options) {
		o.addressBook = book
	}
}

func WithMessageSender(msgSender messageSender) Option {
	return func(o *Options) {
		o.msgSender = msgSender
	}
}

func WithPreviewGenerator(previewGenerator previewGenerator) Option {
	return func(o *Options) {
		o.previewGenerator = previewGenerator
	}
}

func NewHandler(logger *zap.Logger, opts ...Option) (*Handler, error) {
	options := &Options{}
	for _, o := range opts {
		o(options)
	}
	if options.msgSender == nil {
		sender, err := blockchain.NewMsgSender(logger)
		if err != nil {
			return nil, err
		}
		options.msgSender = sender
	}
	if options.addressBook == nil {
		options.addressBook = addressbook.NewAddressBook(logger, config.AddressPath, config.JettonPath, config.CollectionPath)
	}
	if options.chainState == nil {
		options.chainState = chainstate.NewChainState()
	}
	if options.storage == nil {
		return nil, errors.New("storage is not configured")
	}
	return &Handler{
		storage:          options.storage,
		state:            options.chainState,
		addressBook:      options.addressBook,
		msgSender:        options.msgSender,
		previewGenerator: options.previewGenerator,
	}, nil
}

func (h Handler) GetAccount(ctx context.Context, params oas.GetAccountParams) (oas.GetAccountRes, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	info, err := h.storage.GetAccountInfo(ctx, accountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	ab, found := h.addressBook.GetAddressInfoByAddress(accountID)
	if found {
		info.IsScam = &ab.IsScam
		if len(ab.Name) > 0 {
			info.Name = &ab.Name
		}
		if len(ab.Image) > 0 {
			info.Icon = &ab.Image
		}
		info.MemoRequired = &ab.RequireMemo
	}
	res := convertToAccount(info)
	return &res, nil
}

func (h Handler) GetRawAccount(ctx context.Context, params oas.GetRawAccountParams) (r oas.GetRawAccountRes, _ error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	rawAccount, err := h.storage.GetRawAccount(ctx, accountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	res := convertToRawAccount(rawAccount)
	return &res, nil
}

func (h Handler) GetBlock(ctx context.Context, params oas.GetBlockParams) (r oas.GetBlockRes, _ error) {
	id, err := blockIdFromString(params.BlockID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	block, err := h.storage.GetBlockHeader(ctx, id)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.NotFound{Error: "block not found"}, nil
	}
	if err != nil {
		return r, err
	}
	res := convertBlockHeader(*block)
	return &res, nil
}

func (h Handler) GetTransaction(ctx context.Context, params oas.GetTransactionParams) (r oas.GetTransactionRes, _ error) {
	hash, err := tongo.ParseHash(params.TransactionID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	txs, err := h.storage.GetTransaction(ctx, hash)
	if errors.Is(err, core.ErrEntityNotFound) {
		return &oas.NotFound{Error: "transaction not found"}, nil
	}
	if err != nil {
		return nil, err
	}
	transaction := convertTransaction(*txs)
	return &transaction, nil
}

func (h Handler) GetBlockTransactions(ctx context.Context, params oas.GetBlockTransactionsParams) (oas.GetBlockTransactionsRes, error) {
	id, err := blockIdFromString(params.BlockID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	transactions, err := h.storage.GetBlockTransactions(ctx, id)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	res := oas.Transactions{
		Transactions: make([]oas.Transaction, 0, len(transactions)),
	}
	for _, tx := range transactions {
		res.Transactions = append(res.Transactions, convertTransaction(*tx))
	}
	return &res, nil
}

func (h Handler) GetTrace(ctx context.Context, params oas.GetTraceParams) (r oas.GetTraceRes, _ error) {
	hash, err := tongo.ParseHash(params.TraceID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	t, err := h.storage.GetTrace(ctx, hash)
	if err != nil {
		return nil, err
	}
	trace := convertTrace(*t)
	return &trace, nil
}

func (h Handler) PoolsByNominators(ctx context.Context, params oas.PoolsByNominatorsParams) (oas.PoolsByNominatorsRes, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	whalesPools, err := h.storage.GetParticipatingInWhalesPools(ctx, accountID)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	var result oas.AccountStaking
	for _, w := range whalesPools {
		if _, ok := references.WhalesPools[w.Pool]; !ok {
			continue //skip unknown pools
		}
		result.Pools = append(result.Pools, oas.AccountStakingInfo{
			Pool:            w.Pool.ToRaw(),
			Amount:          w.MemberBalance,
			PendingDeposit:  w.MemberPendingDeposit,
			PendingWithdraw: w.MemberPendingWithdraw,
			ReadyWithdraw:   w.MemberWithdraw,
		})
	}
	return &result, nil
}

func (h Handler) StakingPoolInfo(ctx context.Context, params oas.StakingPoolInfoParams) (oas.StakingPoolInfoRes, error) {
	poolID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	if w, prs := references.WhalesPools[poolID]; prs {
		poolConfig, poolStatus, err := h.storage.GetWhalesPoolInfo(ctx, poolID)
		if err != nil {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		return &oas.StakingPoolInfoOK{
			Implementation: oas.PoolImplementation{
				Name:        references.WhalesPoolImplementationsName,
				Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": poolConfig.MinStake / 1_000_000_000}}),
				URL:         references.WhalesPoolImplementationsURL,
			},
			Pool: convertStakingWhalesPool(poolID, w, poolStatus, poolConfig, h.state.GetAPY(), true),
		}, nil
	}
	p, err := h.storage.GetTFPool(ctx, poolID)
	if err != nil {
		return &oas.NotFound{Error: "pool not found: " + err.Error()}, nil
	}

	info, _ := h.addressBook.GetTFPoolInfo(p.Address)

	return &oas.StakingPoolInfoOK{
		Implementation: oas.PoolImplementation{
			Name:        references.TFPoolImplementationsName,
			Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": p.MinNominatorStake / 1_000_000_000}}),
			URL:         references.TFPoolImplementationsURL,
		},
		Pool: convertStakingTFPool(p, info, h.state.GetAPY()),
	}, nil
}

func (h Handler) StakingPools(ctx context.Context, params oas.StakingPoolsParams) (r oas.StakingPoolsRes, _ error) {
	var result oas.StakingPoolsOK

	tfPools, err := h.storage.GetTFPools(ctx)
	if err != nil {
		return nil, err
	}
	var minTF, minWhales int64
	var availableFor *tongo.AccountID
	if params.AvailableFor.IsSet() {
		a, err := tongo.ParseAccountID(params.AvailableFor.Value)
		if err != nil {
			return &oas.BadRequest{Error: err.Error()}, nil
		}
		availableFor = &a
	}
	for _, p := range tfPools {
		if availableFor != nil && p.Nominators >= p.MaxNominators {
			continue
		}
		info, _ := h.addressBook.GetTFPoolInfo(p.Address)
		pool := convertStakingTFPool(p, info, h.state.GetAPY())
		if minTF == 0 || pool.MinStake < minTF {
			minTF = pool.MinStake
		}
		result.Pools = append(result.Pools, pool)
	}

	for k, w := range references.WhalesPools {
		if availableFor != nil {
			_, err = h.storage.GetWhalesPoolMemberInfo(ctx, k, *availableFor)
			if err != nil && !w.AvailableFor(*availableFor) {
				continue
			}
		}
		poolConfig, poolStatus, err := h.storage.GetWhalesPoolInfo(ctx, k)
		if err != nil {
			continue
		}
		pool := convertStakingWhalesPool(k, w, poolStatus, poolConfig, h.state.GetAPY(), true)
		if minWhales == 0 || pool.MinStake < minWhales {
			minWhales = pool.MinStake
		}
		result.Pools = append(result.Pools, pool)
	}

	slices.SortFunc(result.Pools, func(a, b oas.PoolInfo) bool {
		return a.Apy > b.Apy
	})
	result.SetImplementations(map[string]oas.PoolImplementation{
		string(oas.PoolInfoImplementationWhales): {
			Name: references.WhalesPoolImplementationsName,
			Description: i18n.T(params.AcceptLanguage.Value, i18n.C{DefaultMessage: &i18n.M{
				ID:    "poolImplementationDescription",
				Other: "Minimum deposit {{.Deposit}} TON",
			}, TemplateData: map[string]interface{}{"Deposit": minWhales / 1_000_000_000}}),
			URL: references.WhalesPoolImplementationsURL,
		},
		string(oas.PoolInfoImplementationTf): {
			Name:        references.TFPoolImplementationsName,
			Description: i18n.T(params.AcceptLanguage.Value, i18n.C{MessageID: "poolImplementationDescription", TemplateData: map[string]interface{}{"Deposit": minTF / 1_000_000_000}}),
			URL:         references.TFPoolImplementationsURL,
		},
	})

	return &result, nil
}

func (h Handler) GetNftItemsByAddresses(ctx context.Context, params oas.GetNftItemsByAddressesParams) (oas.GetNftItemsByAddressesRes, error) {
	accounts := make([]tongo.AccountID, len(params.AccountIds))
	var err error
	for i := range params.AccountIds {
		accounts[i], err = tongo.ParseAccountID(params.AccountIds[i])
		if err != nil {
			return &oas.BadRequest{Error: err.Error()}, nil
		}
	}
	items, err := h.storage.GetNFTs(ctx, accounts)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	var result oas.NftItems
	for _, i := range items {
		result.NftItems = append(result.NftItems, convertNFT(i))
	}
	return &result, nil
}

func (h Handler) SendMessage(ctx context.Context, req oas.OptSendMessageReq) (r oas.SendMessageRes, _ error) {
	payload, err := base64.StdEncoding.DecodeString(req.Value.Boc)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	if err := h.msgSender.SendMessage(ctx, payload); err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return &oas.SendMessageOK{}, nil
}

func (h Handler) GetJettonsBalances(ctx context.Context, params oas.GetJettonsBalancesParams) (oas.GetJettonsBalancesRes, error) {
	accountID, err := tongo.ParseAccountID(params.AccountID)
	if err != nil {
		return &oas.BadRequest{Error: err.Error()}, nil
	}
	wallets, err := h.storage.GetJettonWalletsByOwnerAddress(ctx, accountID)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	var balances = oas.JettonsBalances{
		Balances: make([]oas.JettonBalance, 0, len(wallets)),
	}
	for _, wallet := range wallets {
		jettonBalance := oas.JettonBalance{
			Balance:       wallet.Balance.String(),
			JettonAddress: wallet.JettonAddress.ToRaw(),
			WalletAddress: convertAccountAddress(wallet.Address),
		}
		meta, err := h.storage.GetJettonMasterMetadata(ctx, wallet.JettonAddress)
		if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
			return &oas.InternalError{Error: err.Error()}, nil
		}
		if !errors.Is(err, core.ErrEntityNotFound) {
			m := convertToApiJetton(meta, h.previewGenerator)
			m.Verification = oas.OptJettonVerificationType{Value: oas.JettonVerificationTypeNone}
			jettonBalance.Metadata = oas.OptJetton{Value: m}
			convertVerification, _ := convertJettonVerification(meta.Verification)
			jettonBalance.Verification = convertVerification
		}

		balances.Balances = append(balances.Balances, jettonBalance)
	}

	return &balances, nil
}
