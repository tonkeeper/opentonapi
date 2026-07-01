package api

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/wallet"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"github.com/tonkeeper/tongo/txemulator"
	tonwallet "github.com/tonkeeper/tongo/wallet"
	"go.uber.org/zap"
)

// migrationGasPerTransfer is attached to every jetton/NFT transfer to cover gas and forwarding.
// Any unused part is reclaimed by the final mode-128 TON sweep, so over-estimating is safe.
const migrationGasPerTransfer = ton.OneGRAM / 20 // 0.05 TON

// migrationForwardAmount is forwarded to the destination so it receives a transfer notification.
const migrationForwardAmount = tlb.Grams(1)

// migrationSweepMode sends the entire remaining balance and ignores errors of the sweep itself.
const migrationSweepMode = 128

const migrationMsgLifetime = 5 * time.Minute

type jettonBulkStorage interface {
	GetJettonWalletsByOwnerAddresses(ctx context.Context, owners []ton.AccountID, mintless bool) ([]core.JettonWallet, error)
}

func (h *Handler) GetMigrationWallets(ctx context.Context, req oas.OptGetMigrationWalletsReq, params oas.GetMigrationWalletsParams) (*oas.MigrationWallets, error) {
	if len(req.Value.AccountIds) == 0 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("empty list of ids"))
	}
	if !h.limits.isBulkQuantityAllowed(len(req.Value.AccountIds)) {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("the maximum number of accounts to request at once: %v", h.limits.BulkLimits))
	}
	ids := make([]tongo.AccountID, 0, len(req.Value.AccountIds))
	for _, v := range req.Value.AccountIds {
		account, err := tongo.ParseAddress(v)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
		ids = append(ids, account.ID)
	}

	var accounts []*core.Account
	nftCountByOwner := make(map[ton.AccountID]int32)
	var jettonWallets map[ton.AccountID][]core.JettonWallet
	var accountsErr, nftsErr, jettonsErr error
	var wg sync.WaitGroup
	wg.Go(func() {
		accounts, accountsErr = h.storage.GetRawAccounts(ctx, ids)
	})
	wg.Go(func() {
		nfts, err := h.storage.GetNFTs(ctx, ids)
		if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
			nftsErr = err
			return
		}
		nftScam, scamErr := h.spamFilter.GetNftsScamData(ctx, ids)
		if scamErr != nil {
			h.logger.Warn("error getting nft scam data", zap.Error(scamErr))
		}
		for _, item := range nfts {
			if item.OwnerAddress == nil {
				continue
			}
			if nftScam[item.Address] == core.TrustBlacklist {
				continue
			}
			nftCountByOwner[*item.OwnerAddress]++
		}
	})
	wg.Go(func() {
		jettonWallets, jettonsErr = h.collectJettonWallets(ctx, ids)
	})
	wg.Wait()
	if accountsErr != nil {
		return nil, toError(http.StatusInternalServerError, accountsErr)
	}
	if nftsErr != nil {
		return nil, toError(http.StatusInternalServerError, nftsErr)
	}
	if jettonsErr != nil {
		return nil, toError(http.StatusInternalServerError, jettonsErr)
	}

	accountByID := make(map[ton.AccountID]*core.Account, len(accounts))
	for _, account := range accounts {
		accountByID[account.AccountAddress] = account
	}
	jettonsByOwner := make(map[ton.AccountID][]oas.JettonBalance, len(jettonWallets))
	for owner, wallets := range jettonWallets {
		balances := make([]oas.JettonBalance, 0, len(wallets))
		for _, w := range wallets {
			if w.Lock != nil {
				// locked jettons cannot be migrated
				continue
			}
			balance, err := h.convertJettonBalance(ctx, w, params.Currencies, nil, nil)
			if err != nil {
				h.logger.Warn(fmt.Sprintf("failed to convert jetton balance for wallet %v", w.JettonAddress.ToRaw()), zap.Error(err))
				continue
			}
			if balance.Jetton.Verification == oas.JettonVerificationTypeBlacklist {
				// skip scam jettons
				continue
			}
			balances = append(balances, balance)
		}
		jettonsByOwner[owner] = balances
	}
	resp := &oas.MigrationWallets{Wallets: make([]oas.MigrationWalletValue, 0, len(ids))}
	for _, id := range ids {
		wallet := oas.MigrationWalletValue{
			Account:  id.ToRaw(),
			Status:   oas.AccountStatusNonexist,
			Jettons:  jettonsByOwner[id],
			NftCount: nftCountByOwner[id],
		}
		if wallet.Jettons == nil {
			wallet.Jettons = []oas.JettonBalance{}
		}
		if account, ok := accountByID[id]; ok {
			wallet.Balance = account.GramBalance
			wallet.Status = oas.AccountStatus(account.Status)
		}
		resp.Wallets = append(resp.Wallets, wallet)
	}
	return resp, nil
}

func getPublicKey(pk oas.OptString) (ed25519.PublicKey, error) {
	if !pk.IsSet() || pk.Value == "" {
		return nil, errors.New("public_key is empty")
	}
	if decoded, err := hex.DecodeString(pk.Value); err != nil {
		return nil, fmt.Errorf("public_key is not valid hex: %v", err)
	} else {
		return decoded, nil
	}
}

func (h *Handler) PrepareMigration(ctx context.Context, req *oas.MigrationPrepareRequest) (*oas.MigrationPrepareResponse, error) {
	from, err := tongo.ParseAddress(req.From)
	if err != nil {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("invalid `from` address: %w", err))
	}
	to, err := tongo.ParseAddress(req.To)
	if err != nil {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("invalid `to` address: %w", err))
	}
	currency := "USD"
	if req.Currency.IsSet() && req.Currency.Value != "" {
		currency = req.Currency.Value
	}
	logger := slog.With(
		slog.String("from", from.ID.String()),
		slog.String("to", to.ID.String()),
		slog.String("pubkey", req.PublicKey.Value),
	)
	var (
		version     tonwallet.Version
		publicKey   ed25519.PublicKey
		deployInit  *tlb.StateInit
		startSeqno  uint32 = 0
		subWalletID uint32 = tonwallet.DefaultSubWallet
	)
	account, err := h.storage.GetRawAccount(ctx, from.ID)
	if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
		logger.Error("error happened on wallet inference", slog.String("error", err.Error()))
		return nil, toError(http.StatusInternalServerError, err)
	} else if err != nil || len(account.Code) == 0 {
		publicKey, err = getPublicKey(req.PublicKey)
		if err != nil {
			return nil, toError(http.StatusBadRequest, fmt.Errorf("source wallet is not initialized; %v", err))
		}
		version, deployInit, err = inferWalletForAddress(from.ID, publicKey)
		if err != nil {
			logger.Error("error happened on wallet inference", slog.String("error", err.Error()))
			return nil, toError(http.StatusInternalServerError, err)
		}
	} else {
		version, err = wallet.GetVersionByCode(account.Code)
		if err != nil {
			return nil, toError(http.StatusBadRequest, fmt.Errorf("unsupported source wallet: %w", err))
		}
		startSeqno, subWalletID, publicKey, err = parseWalletData(version, account.Data)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, fmt.Errorf("can't read wallet data: %w", err))
		}
	}

	// Discover the safe, migratable assets of the source wallet.
	jettons, err := h.collectJettonWallets(ctx, []ton.AccountID{from.ID})
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	nfts, err := h.storage.GetNFTs(ctx, []ton.AccountID{from.ID})
	if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusInternalServerError, err)
	}
	nftScam, scamErr := h.spamFilter.GetNftsScamData(ctx, []ton.AccountID{from.ID})
	if scamErr != nil {
		h.logger.Warn("error getting nft scam data", zap.Error(scamErr))
	}
	// Build the ordered internal messages: jettons, then NFTs, then the final TON sweep.
	var messages []tonwallet.RawMessage
	var gas int64
	for _, jetton := range jettons[from.ID] {
		if jetton.Lock != nil || jetton.Balance.IsZero() {
			continue
		}
		balance, err := h.convertJettonBalance(ctx, jetton, nil, nil, nil)
		if err != nil {
			h.logger.Warn(fmt.Sprintf("skip jetton %v: %v", jetton.JettonAddress.ToRaw(), err))
			continue
		}
		if balance.Jetton.Verification == oas.JettonVerificationTypeBlacklist {
			continue
		}
		msg, err := walletJettonTransferMessage(jetton.Address, to.ID, jetton.Balance.BigInt())
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		msgRaw, err := toWalletRawMessage(msg)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		messages = append(messages, msgRaw)
		gas += int64(migrationGasPerTransfer)
	}
	for _, nft := range nfts {
		if nft.OwnerAddress == nil || *nft.OwnerAddress != from.ID {
			continue
		}
		if nftScam[nft.Address] == core.TrustBlacklist {
			continue
		}
		msg := walletNFTTransferMessage(nft.Address, to.ID)
		msgRaw, err := toWalletRawMessage(msg)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		messages = append(messages, msgRaw)
		gas += int64(migrationGasPerTransfer)
	}
	if account.GramBalance < gas {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("INSUFFICIENT_TON_FOR_GAS: required %d nanotons to cover transfer gas, available %d", gas, account.GramBalance))
	}
	// The final message sweeps the remaining TON balance to the destination.
	msgRaw, err := toWalletRawMessage(tonwallet.Message{
		Amount:  0,
		Address: to.ID,
		Bounce:  false,
		Mode:    migrationSweepMode,
	})
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	messages = append(messages, msgRaw)
	batches := chunkMessages(messages, walletMaxMessageCount(version))
	validUntil := time.Now().Add(migrationMsgLifetime)
	currencyPtr := &currency
	resp := &oas.MigrationPrepareResponse{
		From:          from.ID.ToRaw(),
		To:            to.ID.ToRaw(),
		WalletVersion: version.ToString(),
		Transactions:  make([]oas.MigrationTransaction, 0, len(batches)),
	}
	// Emulate transactions sequentially, feeding each transaction's resulting state into the next so
	// that seqno, balance and emptied jetton wallets are reflected in later fees and previews.
	var overrides map[ton.AccountID]tlb.ShardAccount
	for i, batch := range batches {
		seqno := startSeqno + uint32(i)
		body, err := buildUnsignedBody(version, subWalletID, publicKey, int(from.ID.Workchain), seqno, validUntil, batch)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		bocBase64, err := body.ToBocBase64()
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		signedBody, err := signedBodyForEmulation(version, body)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		var init *tlb.StateInit
		if i == 0 {
			init = deployInit
		}
		extMsg, err := tongo.CreateExternalMessage(from.ID, signedBody, init, tlb.VarUInteger16{})
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		extMsgCell := boc.NewCell()
		if err := tlb.Marshal(extMsgCell, extMsg); err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		risk, err := wallet.ExtractRisk(version, extMsgCell)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		trace, finalStates, err := h.emulateWalletMessage(ctx, extMsg, overrides)
		if err != nil {
			return nil, toProperEmulationError(err)
		}
		convertedTrace := h.convertTrace(trace, h.addressBook)
		actions, err := bath.FindActions(ctx, trace, bath.ForAccount(from.ID), bath.WithInformationSource(h.storage), bath.WithAddressBook(h.addressBook))
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		event, err := h.toAccountEvent(ctx, from.ID, trace, bath.EnrichWithIntentions(trace, actions), oas.OptString{}, true)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		oasRisk, err := h.convertRisk(ctx, *risk, from.ID, currencyPtr)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		transaction := oas.MigrationTransaction{
			Seqno: int32(seqno),
			Boc:   bocBase64,
			Emulation: oas.MessageConsequences{
				Trace: convertedTrace,
				Event: event,
				Risk:  oasRisk,
			},
		}
		if init != nil {
			transaction.StateInit, err = convertStateInit(*init)
			if err != nil {
				return nil, toError(http.StatusInternalServerError, err)
			}
		}
		resp.Transactions = append(resp.Transactions, transaction)
		overrides = finalStates
	}
	return resp, nil
}

func convertStateInit(si tlb.StateInit) (oas.OptString, error) {
	cell := boc.NewCell()
	if err := tlb.Marshal(cell, si); err != nil {
		return oas.OptString{}, fmt.Errorf("marshalling stat init: %v", err)
	}
	b64, err := cell.ToBocBase64()
	if err != nil {
		return oas.OptString{}, fmt.Errorf("base64 encoding failed: %v", err)
	}
	return oas.NewOptString(b64), nil
}

func (h *Handler) collectJettonWallets(ctx context.Context, owners []ton.AccountID) (map[ton.AccountID][]core.JettonWallet, error) {
	bulk, ok := h.storage.(jettonBulkStorage)
	if !ok {
		return nil, core.ErrEntityNotFound
	}
	wallets, err := bulk.GetJettonWalletsByOwnerAddresses(ctx, owners, false)
	if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
		return nil, err
	}
	byOwner := make(map[ton.AccountID][]core.JettonWallet, len(owners))
	for _, w := range wallets {
		if w.OwnerAddress == nil {
			continue
		}
		byOwner[*w.OwnerAddress] = append(byOwner[*w.OwnerAddress], w)
	}
	return byOwner, nil
}

func (h *Handler) emulateWalletMessage(ctx context.Context, msg tlb.Message, overrides map[ton.AccountID]tlb.ShardAccount) (*core.Trace, map[ton.AccountID]tlb.ShardAccount, error) {
	configBase64, err := h.storage.TrimmedConfigBase64()
	if err != nil {
		return nil, nil, err
	}
	options := []txemulator.TraceOption{
		txemulator.WithConfigBase64(configBase64),
		txemulator.WithAccountsSource(h.storage),
		txemulator.WithLimit(1100),
		txemulator.WithIgnoreSignatureDepth(1),
	}
	if len(overrides) > 0 {
		options = append(options, txemulator.WithAccountsMap(overrides))
	}
	emulator, err := txemulator.NewTraceBuilder(options...)
	if err != nil {
		return nil, nil, err
	}
	tree, err := emulator.Run(ctx, msg)
	if err != nil {
		return nil, nil, err
	}
	trace, err := EmulatedTreeToTrace(ctx, h.executor, h.storage, tree, emulator.FinalStates(), nil, h.configPool, true)
	if err != nil {
		return nil, nil, err
	}
	return trace, emulator.FinalStates(), nil
}

func parseWalletData(version tonwallet.Version, data []byte) (seqno uint32, subWalletID uint32, publicKey ed25519.PublicKey, err error) {
	cells, err := boc.DeserializeBoc(data)
	if err != nil {
		return 0, 0, nil, err
	}
	if len(cells) == 0 {
		return 0, 0, nil, fmt.Errorf("empty wallet data")
	}
	switch version {
	case tonwallet.V3R1, tonwallet.V3R2:
		var d tonwallet.DataV3
		if err := tlb.Unmarshal(cells[0], &d); err != nil {
			return 0, 0, nil, err
		}
		return d.Seqno, d.SubWalletId, append(ed25519.PublicKey{}, d.PublicKey[:]...), nil
	case tonwallet.V4R1, tonwallet.V4R2:
		var d tonwallet.DataV4
		if err := tlb.Unmarshal(cells[0], &d); err != nil {
			return 0, 0, nil, err
		}
		return d.Seqno, d.SubWalletId, append(ed25519.PublicKey{}, d.PublicKey[:]...), nil
	case tonwallet.V5R1, tonwallet.V5Beta:
		var d tonwallet.DataV5R1
		if err := tlb.Unmarshal(cells[0], &d); err != nil {
			return 0, 0, nil, err
		}
		return d.Seqno, 0, append(ed25519.PublicKey{}, d.PublicKey[:]...), nil
	default:
		return 0, 0, nil, fmt.Errorf("unsupported wallet version for migration: %v", version.ToString())
	}
}

func inferWalletForAddress(address ton.AccountID, pubkey ed25519.PublicKey) (tonwallet.Version, *tlb.StateInit, error) {
	workchain := int(address.Workchain)
	for _, v := range []tonwallet.Version{
		tonwallet.V5Beta,
		tonwallet.V5R1,
		tonwallet.V4R2,
		tonwallet.V4R1,
		tonwallet.V3R2,
		tonwallet.V3R1,
	} {
		guess, err := tonwallet.GenerateWalletAddress(pubkey, v, nil, workchain, nil)
		if err != nil || guess != address {
			continue
		}
		init, err := tonwallet.GenerateStateInit(pubkey, v, nil, workchain, nil)
		if err != nil {
			return 0, nil, fmt.Errorf("can't build state init for %v: %w", v.ToString(), err)
		}
		return v, &init, nil
	}
	return 0, nil, fmt.Errorf("can't determine source wallet version from the provided pubkey")
}

func buildUnsignedBody(v tonwallet.Version, subWalletID uint32, pubkey ed25519.PublicKey, workchain int, seqno uint32, validUntil time.Time, s []tonwallet.RawMessage) (*boc.Cell, error) {
	switch v {
	case tonwallet.V3R1, tonwallet.V3R2:
		body := tonwallet.MessageV3{
			SubWalletId: subWalletID,
			ValidUntil:  uint32(validUntil.Unix()),
			Seqno:       seqno,
			RawMessages: tonwallet.PayloadV1toV4(s),
		}
		cell := boc.NewCell()
		if err := tlb.Marshal(cell, body); err != nil {
			return nil, err
		}
		return cell, nil
	case tonwallet.V4R1, tonwallet.V4R2:
		body := tonwallet.MessageV4{
			SubWalletId: subWalletID,
			ValidUntil:  uint32(validUntil.Unix()),
			Seqno:       seqno,
			Op:          0,
			RawMessages: tonwallet.PayloadV1toV4(s),
		}
		cell := boc.NewCell()
		if err := tlb.Marshal(cell, body); err != nil {
			return nil, err
		}
		return cell, nil
	case tonwallet.V5R1, tonwallet.V5Beta:
		w5 := tonwallet.NewWalletV5R1(pubkey, tonwallet.Options{Workchain: &workchain})
		return w5.CreateMsgBodyWithoutSignature(s, tonwallet.MessageConfig{
			Seqno:      seqno,
			ValidUntil: validUntil,
			V5MsgType:  tonwallet.V5MsgTypeSignedExternal,
		})
	default:
		return nil, fmt.Errorf("unsupported wallet version for migration: %v", v.ToString())
	}
}

// signedBodyForEmulation wraps the unsigned body with a zero signature so it can be emulated with
// signature checks disabled. v3/v4 prepend the signature, v5 carries a trailing placeholder.
func signedBodyForEmulation(v tonwallet.Version, body *boc.Cell) (*boc.Cell, error) {
	switch v {
	case tonwallet.V5R1, tonwallet.V5Beta:
		return body, nil
	default:
		signed := tonwallet.SignedMsgBody{Sign: tlb.Bits512{}, Message: tlb.Any(*body)}
		cell := boc.NewCell()
		if err := tlb.Marshal(cell, signed); err != nil {
			return nil, err
		}
		return cell, nil
	}
}

func chunkMessages(s []tonwallet.RawMessage, n int) [][]tonwallet.RawMessage {
	if n <= 0 {
		n = 1
	}
	var batches [][]tonwallet.RawMessage
	for i := 0; i < len(s); i += n {
		end := min(i+n, len(s))
		batches = append(batches, s[i:end])
	}
	return batches
}

func walletJettonTransferMessage(src, dst ton.AccountID, amount *big.Int) (tonwallet.Message, error) {
	body := boc.NewCell()
	msgBody := abi.JettonTransferMsgBody{
		QueryId:             0,
		Amount:              tlb.VarUInteger16(*amount),
		Destination:         dst.ToMsgAddress(),
		ResponseDestination: dst.ToMsgAddress(),
		ForwardTonAmount:    tlb.VarUInteger16(*big.NewInt(int64(migrationForwardAmount))),
	}
	if err := body.WriteUint(0xf8a7ea5, 32); err != nil {
		return tonwallet.Message{}, err
	}
	if err := tlb.Marshal(body, msgBody); err != nil {
		return tonwallet.Message{}, err
	}
	return tonwallet.Message{
		Amount:  migrationGasPerTransfer,
		Address: src,
		Bounce:  true,
		Mode:    tonwallet.DefaultMessageMode,
		Body:    body,
	}, nil
}

func walletNFTTransferMessage(src, dst ton.AccountID) tonwallet.Message {
	body := boc.NewCell()
	msgBody := abi.NftTransferMsgBody{
		QueryId:             0,
		NewOwner:            dst.ToMsgAddress(),
		ResponseDestination: dst.ToMsgAddress(),
		ForwardAmount:       tlb.VarUInteger16(*big.NewInt(int64(migrationForwardAmount))),
	}
	_ = body.WriteUint(0x5fcc3d14, 32)
	_ = tlb.Marshal(body, msgBody)
	return tonwallet.Message{
		Amount:  migrationGasPerTransfer,
		Address: src,
		Bounce:  true,
		Mode:    tonwallet.DefaultMessageMode,
		Body:    body,
	}
}

func toWalletRawMessage(msg tonwallet.Message) (tonwallet.RawMessage, error) {
	intMsg, mode, err := msg.ToInternal()
	if err != nil {
		return tonwallet.RawMessage{}, err
	}
	cell := boc.NewCell()
	if err := tlb.Marshal(cell, intMsg); err != nil {
		return tonwallet.RawMessage{}, err
	}
	return tonwallet.RawMessage{Message: cell, Mode: mode}, nil
}

func walletMaxMessageCount(v tonwallet.Version) int {
	switch v {
	case tonwallet.V5R1, tonwallet.V5Beta:
		return 255
	default:
		return 4
	}
}
