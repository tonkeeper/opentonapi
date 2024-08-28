package api

import (
	"context"
	"encoding/base64"
	"errors"
	"math/big"
	"sync"
	"time"

	"golang.org/x/exp/slices"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"github.com/tonkeeper/tongo/txemulator"
	tongoWallet "github.com/tonkeeper/tongo/wallet"

	"github.com/tonkeeper/opentonapi/pkg/blockchain"
	"github.com/tonkeeper/opentonapi/pkg/cache"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/sentry"
)

var (
	emulatedMessagesCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tonapi_emulated_messages_counter",
		Help: "The total number of emulated messages",
	})
	emulatedAccountCode = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "tonapi_emulated_account_code_counter",
	}, []string{"code_hash"})
)

func (h *Handler) RunEmulation(ctx context.Context, msgCh <-chan blockchain.ExtInMsgCopy, emulationCh chan<- blockchain.ExtInMsgCopy) {
	for {
		select {
		case <-ctx.Done():
			return
		case msgCopy := <-msgCh:
			emulatedMessagesCounter.Inc()
			go func() {
				ctx, cancel := context.WithTimeout(ctx, time.Second*5)
				defer cancel()

				// TODO: find a way to emulate when tonapi receives a batch of messages in a single request to SendBlockchainMessage endpoint.
				_, err := h.addToMempool(ctx, msgCopy.Payload, nil, emulationCh)
				if err != nil {
					sentry.Send("addToMempool", sentry.SentryInfoData{"payload": msgCopy.Payload}, sentry.LevelError)
				}
			}()
		}
	}
}

func isHighload(version tongoWallet.Version) bool {
	return version == tongoWallet.HighLoadV1R1 ||
		version == tongoWallet.HighLoadV1R2 ||
		version == tongoWallet.HighLoadV2R1 ||
		version == tongoWallet.HighLoadV2R2 ||
		version == tongoWallet.HighLoadV2
}

func (h *Handler) isEmulationAllowed(accountID ton.AccountID, state tlb.ShardAccount, m tlb.Message) (bool, error) {
	if _, ok := h.addressBook.GetAddressInfoByAddress(accountID); ok {
		return true, nil
	}
	version, ok, err := tongoWallet.GetWalletVersion(state, m)
	if err != nil {
		return false, err
	}
	if ok && isHighload(version) {
		return false, nil
	}
	return true, nil
}

func (h *Handler) addToMempool(ctx context.Context, bytesBoc []byte, shardAccount map[tongo.AccountID]tlb.ShardAccount, emulationCh chan<- blockchain.ExtInMsgCopy) (map[tongo.AccountID]tlb.ShardAccount, error) {
	if shardAccount == nil {
		shardAccount = map[tongo.AccountID]tlb.ShardAccount{}
	}
	msgCell, err := boc.DeserializeBoc(bytesBoc)
	if err != nil {
		return shardAccount, err
	}

	ttl := int64(30)
	msgV4, err := tongoWallet.DecodeMessageV4(msgCell[0])
	if err == nil {
		diff := int64(msgV4.ValidUntil) - time.Now().Unix()
		if diff < 600 {
			ttl = diff
		}
	}
	var message tlb.Message
	err = tlb.Unmarshal(msgCell[0], &message)
	if err != nil {
		return shardAccount, err
	}
	walletAddress, err := extractDestinationWallet(message)
	if err != nil {
		return nil, err
	}
	state, err := h.storage.GetAccountState(ctx, *walletAddress)
	if err != nil {
		return nil, err
	}
	allowed, err := h.isEmulationAllowed(*walletAddress, state, message)
	if err != nil {
		return shardAccount, err
	}
	if !allowed {
		return shardAccount, nil
	}
	config, err := h.storage.TrimmedConfigBase64()
	if err != nil {
		return shardAccount, err
	}
	emulator, err := txemulator.NewTraceBuilder(txemulator.WithAccountsSource(h.storage),
		txemulator.WithAccountsMap(shardAccount),
		txemulator.WithConfigBase64(config),
		txemulator.WithSignatureCheck(),
	)
	if err != nil {
		return shardAccount, err
	}
	tree, err := emulator.Run(ctx, message)
	if err != nil {
		return shardAccount, err
	}
	newShardAccount := emulator.FinalStates()
	trace, err := emulatedTreeToTrace(ctx, h.executor, h.storage, tree, newShardAccount, nil, h.configPool)
	if err != nil {
		return shardAccount, err
	}
	accounts := make(map[tongo.AccountID]struct{})
	core.Visit(trace, func(node *core.Trace) {
		accounts[node.Account] = struct{}{}
	})
	hash, err := msgCell[0].Hash256()
	if err != nil {
		return shardAccount, err
	}
	h.mempoolEmulate.traces.Set(hash, trace, cache.WithExpiration(time.Second*time.Duration(ttl)))
	var localMessageHashCache = make(map[ton.Bits256]bool)
	for account := range accounts {
		if _, ok := h.mempoolEmulateIgnoreAccounts[account]; ok { // the map is filled only once at the start
			continue
		}
		oldMemHashes, _ := h.mempoolEmulate.accountsTraces.Get(account)
		newMemHashes := make([]ton.Bits256, 0, len(oldMemHashes)+1)
		for _, mHash := range oldMemHashes { //we need to filter messages which already created transactions
			includedToDB, prs := localMessageHashCache[mHash]
			if !prs {
				_, err = h.storage.SearchTransactionByMessageHash(ctx, mHash)
				includedToDB = err == nil
				localMessageHashCache[mHash] = includedToDB
			}
			_, existsInCache := h.mempoolEmulate.traces.Get(mHash)
			if !includedToDB && existsInCache { //because if err is not null it already happened and if !prs it is not in mempool
				newMemHashes = append(newMemHashes, mHash)
			}
		}
		newMemHashes = append(newMemHashes, hash) // it's important to make it last
		h.mempoolEmulate.accountsTraces.Set(account, newMemHashes, cache.WithExpiration(time.Second*time.Duration(ttl)))
	}
	emulationCh <- blockchain.ExtInMsgCopy{
		MsgBoc:   base64.StdEncoding.EncodeToString(bytesBoc),
		Details:  h.ctxToDetails(ctx),
		Payload:  bytesBoc,
		Accounts: accounts,
	}
	return newShardAccount, nil
}

func emulatedTreeToTrace(
	ctx context.Context,
	executor executor,
	resolver core.LibraryResolver,
	tree *txemulator.TxTree,
	accounts map[tongo.AccountID]tlb.ShardAccount,
	inspectionCache map[ton.AccountID]*abi.ContractDescription,
	configPool *sync.Pool,
) (*core.Trace, error) {
	if !tree.TX.Msgs.InMsg.Exists {
		return nil, errors.New("there is no incoming message in emulation result")
	}
	if inspectionCache == nil {
		inspectionCache = make(map[ton.AccountID]*abi.ContractDescription)
	}
	m := tree.TX.Msgs.InMsg.Value.Value
	var a tlb.MsgAddress
	switch m.Info.SumType {
	case "IntMsgInfo":
		a = m.Info.IntMsgInfo.Dest
	case "ExtInMsgInfo":
		a = m.Info.ExtInMsgInfo.Dest
	default:
		return nil, errors.New("unknown message type in emulation result")
	}
	transaction, err := core.ConvertTransaction(int32(a.AddrStd.WorkchainId), tongo.Transaction{
		Transaction: tree.TX,
		BlockID:     tongo.BlockIDExt{BlockID: tongo.BlockID{Workchain: int32(a.AddrStd.WorkchainId)}},
	})
	filteredMsgs := make([]core.Message, 0, len(transaction.OutMsgs))
	for _, msg := range transaction.OutMsgs {
		if msg.Destination == nil {
			filteredMsgs = append(filteredMsgs, msg)
		}
	}
	transaction.OutMsgs = filteredMsgs //all internal messages in emulation result are delivered to another account and created transaction
	if err != nil {
		return nil, err
	}
	t := &core.Trace{
		Transaction: *transaction,
	}
	additionalInfo := &core.TraceAdditionalInfo{}
	for i := range tree.Children {
		child, err := emulatedTreeToTrace(ctx, executor, resolver, tree.Children[i], accounts, inspectionCache, configPool)
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, child)
	}
	accountID := t.Account
	code := accountCode(accounts[accountID])
	if code == nil {
		return t, nil
	}
	b, err := code.ToBoc()
	if err != nil {
		return nil, err
	}
	codeHash, err := code.HashString()
	if err != nil {
		return nil, err
	}
	emulatedAccountCode.WithLabelValues(codeHash).Inc()
	sharedExecutor := newSharedAccountExecutor(accounts, executor, resolver, configPool)
	inspectionResult, ok := inspectionCache[accountID]
	if !ok {
		inspectionResult, err = abi.NewContractInspector(abi.InspectWithLibraryResolver(resolver)).InspectContract(ctx, b, sharedExecutor, accountID)
		if err != nil {
			return nil, err
		}
		inspectionCache[accountID] = inspectionResult
	}

	// TODO: for all obtained Jetton Masters confirm that jetton wallets are valid
	t.AccountInterfaces = inspectionResult.ContractInterfaces
	for _, m := range inspectionResult.GetMethods {
		switch data := m.Result.(type) {
		case abi.GetNftDataResult:
			if !slices.Contains(inspectionResult.ContractInterfaces, abi.Teleitem) {
				continue
			}
			value := big.Int(data.Index)
			index := decimal.NewFromBigInt(&value, 0)
			collectionAddr, err := ton.AccountIDFromTlb(data.CollectionAddress)
			if err != nil || collectionAddr == nil {
				continue
			}
			_, nftByIndex, err := abi.GetNftAddressByIndex(ctx, sharedExecutor, *collectionAddr, data.Index)
			if err != nil {
				continue
			}
			indexResult, ok := nftByIndex.(abi.GetNftAddressByIndexResult)
			if !ok {
				continue
			}
			nftAddr, err := ton.AccountIDFromTlb(indexResult.Address)
			if err != nil || nftAddr == nil {
				continue
			}
			additionalInfo.EmulatedTeleitemNFT = &core.EmulatedTeleitemNFT{
				Index:             index,
				CollectionAddress: collectionAddr,
				Verified:          *nftAddr == accountID,
			}
		case abi.GetWalletDataResult:
			master, _ := ton.AccountIDFromTlb(data.Jetton)
			additionalInfo.SetJettonMaster(accountID, *master)
		case abi.GetSaleData_GetgemsResult:
			price := big.Int(data.FullPrice)
			owner, err := ton.AccountIDFromTlb(data.Owner)
			if err != nil {
				continue
			}
			item, err := ton.AccountIDFromTlb(data.Nft)
			if err != nil || item == nil {
				continue
			}
			additionalInfo.NftSaleContract = &core.NftSaleContract{
				NftPrice: price.Int64(),
				Owner:    owner,
				Item:     *item,
			}
		case abi.GetSaleData_BasicResult:
			price := big.Int(data.FullPrice)
			owner, err := ton.AccountIDFromTlb(data.Owner)
			if err != nil {
				continue
			}
			item, err := ton.AccountIDFromTlb(data.Nft)
			if err != nil || item == nil {
				continue
			}
			additionalInfo.NftSaleContract = &core.NftSaleContract{
				NftPrice: price.Int64(),
				Owner:    owner,
				Item:     *item,
			}
		case abi.GetSaleData_GetgemsAuctionResult:
			owner, err := ton.AccountIDFromTlb(data.Owner)
			if err != nil {
				continue
			}
			item, err := ton.AccountIDFromTlb(data.Nft)
			if err != nil || item == nil {
				continue
			}
			additionalInfo.NftSaleContract = &core.NftSaleContract{
				NftPrice: int64(data.MaxBid),
				Owner:    owner,
				Item:     *item,
			}
		case abi.GetPoolData_StonfiResult:
			t0, err0 := ton.AccountIDFromTlb(data.Token0Address)
			t1, err1 := ton.AccountIDFromTlb(data.Token1Address)
			if err1 != nil || err0 != nil {
				continue
			}
			additionalInfo.STONfiPool = &core.STONfiPool{
				Token0: *t0,
				Token1: *t1,
			}
			for _, accountID := range []ton.AccountID{*t0, *t1} {
				_, value, err := abi.GetWalletData(ctx, sharedExecutor, accountID)
				if err != nil {
					return nil, err
				}
				data := value.(abi.GetWalletDataResult)
				master, _ := ton.AccountIDFromTlb(data.Jetton)
				additionalInfo.SetJettonMaster(accountID, *master)
			}
		}
	}
	t.SetAdditionalInfo(additionalInfo)
	return t, nil
}
