package api

import (
	"cmp"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/bath"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/gasless"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/opentonapi/pkg/references"
	"github.com/tonkeeper/opentonapi/pkg/wallet"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/contract/jetton"
	"github.com/tonkeeper/tongo/contract/nft"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"github.com/tonkeeper/tongo/txemulator"
	"github.com/tonkeeper/tongo/utils"
	tonwallet "github.com/tonkeeper/tongo/wallet"
	"go.uber.org/zap"
)

// migrationGasPerTransfer is attached to every jetton/NFT transfer to cover gas and forwarding.
// Any unused part is reclaimed by the final mode-128 TON sweep, so over-estimating is safe.
const migrationGasPerTransfer = ton.OneGRAM / 20 // 0.05 TON

// migrationForwardAmount is forwarded to the destination so it receives a transfer notification.
const migrationForwardAmount = tlb.Grams(1)

// minGramTransferFee is threshold below which "send remaining gram" does not make sense
const minGramTransferFee = 500000

// migrationSweepMode sends the entire remaining balance (128) and ignores errors of the sweep
// itself (+2). The +2 SendIgnoreErrors bit is mandatory for wallet v5: it rejects any action sent
// from an external message that lacks it (exit code 137), which would otherwise revert the whole
// batch. v3/v4 don't enforce it, but the bit is harmless there.
const migrationSweepMode = 128 + 2

// migrationOverfundMargin is added on top of the plan's minimum gas when the source is over-funded
// for emulation. The minimum only counts what is attached to the transfers; the margin covers what
// the wallet transactions themselves burn (compute, forwarding, storage) so that a wallet which can
// really afford the migration never fails emulation on a rounding difference. It never reaches the
// response: dropOverfundFromBalances and the pre-sweep deduction take it back out.
const migrationOverfundMargin = 5 * ton.OneGRAM

// migrationMsgLifetime is how long a single prepared transaction stays valid. It is counted from the
// point at which that transaction's turn comes, not from the moment prepare was called — see
// migrationValidUntil.
const migrationMsgLifetime = 5 * time.Minute

// migrationNftPageSize is the SearchNFTs page size used when enumerating a wallet's NFTs for migration.
const migrationNftPageSize = 1000

// batterySponsorshipCap see MaxHelp = 2.1 TON in custodial-battery
const batterySponsorshipCap = 2_100_000_000

// sponsoredBatchMaxMessages bounds a battery-sponsored batch so its gas fits the sponsorship cap.
const sponsoredBatchMaxMessages = int(batterySponsorshipCap / migrationGasPerTransfer)

var migrationSkipNftCollections = map[ton.AccountID]bool{
	references.TonstakersAccountPool: true,
}

// errEmulationFailed marks a failure of the emulator run itself, as opposed to an infrastructure
// error in the surrounding emulation setup. See toMigrationEmulationError.
var errEmulationFailed = errors.New("migration batch emulation failed")

var errJettonNotAvailableForMigration = errors.New("jetton not available for migration")

type jettonConverter func(core.JettonWallet) (oas.JettonBalance, error)

func isSkippedNftCollection(collection *ton.AccountID) bool {
	if collection == nil {
		return false
	}
	return migrationSkipNftCollections[*collection]
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
		var nfts []core.NftItem
		for _, id := range ids {
			owned, err := h.collectOwnedNFTs(ctx, id)
			if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
				nftsErr = err
				return
			}
			nfts = append(nfts, owned...)
		}
		nftItemIDs := make([]ton.AccountID, 0, len(nfts))
		for _, item := range nfts {
			nftItemIDs = append(nftItemIDs, item.Address)
		}
		nftScam, scamErr := h.spamFilter.GetNftsScamData(ctx, nftItemIDs)
		if scamErr != nil {
			h.logger.Warn("error getting nft scam data", zap.Error(scamErr))
		}
		for _, item := range nfts {
			if item.OwnerAddress == nil {
				continue
			}
			if !h.isWhitelistedNft(ctx, item, nftScam[item.Address]) {
				continue
			}
			if isSkippedNftCollection(item.CollectionAddress) {
				continue
			}
			if !item.Transferable {
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
		for _, migration := range h.getJettonMigrations(ctx, wallets, params.Currencies) {
			balance := migration.balance
			balances = append(balances, balance)
		}
		jettonsByOwner[owner] = balances
	}
	resp := &oas.MigrationWallets{Wallets: make([]oas.MigrationWalletValue, 0, len(ids))}
	for _, id := range ids {
		w := oas.MigrationWalletValue{
			Account:  id.ToRaw(),
			Status:   oas.AccountStatusNonexist,
			Jettons:  jettonsByOwner[id],
			NftCount: nftCountByOwner[id],
		}
		if w.Jettons == nil {
			w.Jettons = []oas.JettonBalance{}
		}
		if account, ok := accountByID[id]; ok {
			w.Balance = account.GramBalance
			w.Status = oas.AccountStatus(account.Status)
		}
		if !hasMigratableAssets(w) {
			continue
		}
		resp.Wallets = append(resp.Wallets, w)
	}
	// Best-effort: used only to rank wallets by fiat value below. On failure every
	// jetton contributes 0, degrading the ranking to TON balance alone.
	todayRates, _, _, _, ratesErr := h.getRates()
	if todayRates == nil || ratesErr != nil {
		h.logger.Warn("migration: can't get rates for fiat sort", zap.Error(ratesErr))
	} else {
		// Rank by fiat value: a TON-denominated total (gram balance plus jetton
		// holdings priced via todayRates) sorts the same as a fiat amount would, since
		// every wallet's total would be divided by the same currency price.
		fiatValue := make(map[string]float64, len(resp.Wallets))
		for _, w := range resp.Wallets {
			total := float64(w.Balance) / float64(ton.OneGRAM)
			for _, jb := range w.Jettons {
				rate, ok := todayRates[jb.Jetton.Address]
				if !ok || rate == 0 {
					continue
				}
				amount, ok := new(big.Float).SetString(jb.Balance)
				if !ok {
					continue
				}
				human := new(big.Float).Quo(amount, big.NewFloat(math.Pow10(jb.Jetton.Decimals)))
				f, _ := new(big.Float).Mul(human, big.NewFloat(rate)).Float64()
				total += f
			}
			fiatValue[w.Account] = total
		}
		slices.SortFunc(resp.Wallets, func(a, b oas.MigrationWalletValue) int {
			return cmp.Compare(fiatValue[b.Account], fiatValue[a.Account])
		})
	}
	return resp, nil
}

type migrationBatch struct {
	messages   []tonwallet.RawMessage
	sponsored  bool
	commission *big.Int // fee set by gasless estimate
}

type migrationPlan []migrationBatch

func (mb migrationBatch) chunk(n int) (batches []migrationBatch) {
	for b := range slices.Chunk(mb.messages, n) {
		batches = append(batches, migrationBatch{
			messages:  b,
			sponsored: mb.sponsored,
		})
	}
	return
}

func (mb migrationBatch) gasFundedMessages() int {
	var n int
	for _, message := range mb.messages {
		if message.Mode&128 == 0 {
			n++
		}
	}
	return n
}

func (plan migrationPlan) minGramBalanceRequired() tlb.Grams {
	var res tlb.Grams
	for _, batch := range plan {
		if !batch.sponsored {
			res += tlb.Grams(batch.gasFundedMessages()) * migrationGasPerTransfer
		}
	}
	return res
}

func (h *Handler) PrepareMigration(ctx context.Context, req *oas.MigrationPrepareRequest) (oas.PrepareMigrationRes, error) {
	sourceAddr, err := tongo.ParseAddress(req.From)
	if err != nil {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("invalid `from` address: %w", err))
	}
	destAddr, err := tongo.ParseAddress(req.To)
	if err != nil {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("invalid `to` address: %w", err))
	}
	currency := "USD"
	if req.Currency.IsSet() && req.Currency.Value != "" {
		currency = req.Currency.Value
	}
	gasPayer := req.GasPayer.Or(oas.MigrationPrepareRequestGasPayerSelf)
	batteryPays := gasPayer == oas.MigrationPrepareRequestGasPayerBattery
	gaslessPays := gasPayer == oas.MigrationPrepareRequestGasPayerGasless
	if (batteryPays || gaslessPays) && h.gasless == nil {
		return nil, toError(http.StatusNotImplemented, fmt.Errorf("gas_payer %v is not supported by this deployment", gasPayer))
	}
	logger := slog.With(
		slog.String("from", sourceAddr.ID.String()),
		slog.String("to", destAddr.ID.String()),
		slog.String("pubkey", req.PublicKey.Value),
	)
	sourceAccount, err := h.storage.GetLatestAccountState(ctx, sourceAddr.ID)
	if err != nil {
		logger.Error("failed to get account state", slog.String("error", err.Error()))
		return nil, toError(http.StatusInternalServerError, err)
	}
	sourceWallet, startSeqno, deployInit, err := resolveWallet(sourceAccount, sourceAddr.ID, req.PublicKey)
	if err != nil {
		logger.Error("error happened on wallet inference", slog.String("error", err.Error()))
		return nil, err
	}
	// both relayed modes need owner-signed internal messages, which only v5 supports
	if (gaslessPays || batteryPays) && !sourceWallet.IsRelaySupported() {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("%v migration requires a v5 source wallet; use gas_payer=self", gasPayer))
	}
	var gasJettonMaster *ton.AccountID
	if gaslessPays {
		gasJettonMaster, err = h.requireGasJetton(ctx, req.GasJettonMaster)
		if err != nil {
			return nil, toError(http.StatusBadRequest, err)
		}
	}
	// todo allow battery for v4 and v3 by pre-funding the source instead of relaying
	relayFunded := batteryPays || gaslessPays
	var gramBalance tlb.Grams
	if currColl, ok := sourceAccount.Account.CurrencyCollection(); ok {
		gramBalance = currColl.Grams
	}

	// Fetch rates once for the fiat previews below. Best-effort: on failure we skip the fiat suffix.
	todayRates, _, _, _, ratesErr := h.getRates()
	var currencyPtr *string
	if ratesErr != nil {
		h.logger.Warn("migration: can't get rates for fiat preview", zap.Error(ratesErr))
	} else {
		if _, ok := todayRates[strings.ToUpper(currency)]; ok {
			currencyPtr = &currency
		}
	}

	plan, err := h.prepareMigrationPlan(ctx, sourceAddr, destAddr, relayFunded, gramBalance, sourceWallet, gasJettonMaster)
	if err != nil {
		logger.Error("failed to prepare migration plan", slog.String("error", err.Error()))
		return nil, toError(http.StatusInternalServerError, err)
	}

	minGramBalance := plan.minGramBalanceRequired()

	resp := &oas.MigrationPrepareResponse{
		From:          sourceAddr.ID.ToRaw(),
		To:            destAddr.ID.ToRaw(),
		WalletVersion: sourceWallet.GetVersion().ToString(),
		Transactions:  make([]oas.MigrationTransaction, 0, len(plan)),
	}
	// TODO: test that initial balance is always correct for uninit wallets
	var emulationOverfund tlb.Grams
	emulationState := make(map[ton.AccountID]tlb.ShardAccount)
	emulationStartBalance := gramBalance
	if minGramBalance != 0 && sourceAccount.Account.SumType == "Account" {
		emulationOverfund = minGramBalance + migrationOverfundMargin
		emulationStartBalance += emulationOverfund
		stateOverfund := sourceAccount // copy
		stateOverfund.Account.Account.Storage.Balance.Grams = emulationStartBalance
		emulationState[sourceAddr.ID] = stateOverfund
	} else {
		emulationState[sourceAddr.ID] = sourceAccount
	}
	// A source that can't cover the gas still gets the whole plan: emulation runs against the
	// over-funded balance, so every batch can be built and previewed, and the shortfall is reported
	// together with that preview once the loop is done.
	gramShortfall := worseShortfall(nil, int64(minGramBalance), int64(gramBalance))
	if gramShortfall != nil && emulationOverfund == 0 {
		// Nothing was over-funded — the source is not initialized — so emulation would genuinely run
		// out of funds and report a batch failure instead of the shortfall. Report it bare.
		return nil, errInsufficientGramForGas(gramShortfall.Required, gramShortfall.Available)
	}
	// fail degrades a later failure to the shortfall that caused it. Once the source is known to be
	// short, a broken batch says nothing new, and half a plan must never be handed out: the client
	// signs the transactions in order, so a truncated list would sweep the TON before the jettons.
	fail := func(err error) (oas.PrepareMigrationRes, error) {
		if gramShortfall != nil {
			return nil, errInsufficientGramForGas(gramShortfall.Required, gramShortfall.Available)
		}
		return nil, err
	}
	emuTime := time.Now().Unix()
	seqno := startSeqno
	for _, batch := range plan {
		// Every batch gets its own deadline, counted from the clock this batch starts on rather than
		// from the moment prepare was called. Both consumers need that. Emulation advances its clock by
		// the chain time each batch takes (shard delays, several rounds per jetton/NFT transfer), and
		// the client signs and broadcasts the batches in order, waiting for each to land before the
		// next. A wallet that only fits four messages per transaction — v3 and v4 — turns a wallet with
		// a few dozen assets into a dozen-plus batches, and a single now+lifetime deadline shared by all
		// of them expires partway down the plan: the wallet then rejects the remaining batches with exit
		// code 36 (valid_until <= now), which surfaces here as an emulation failure in the middle of an
		// otherwise fine migration.
		validUntil := migrationValidUntil(emuTime)
		// the final TON sweep carries no gas-funded messages
		sweep := batch.gasFundedMessages() == 0
		if emulationOverfund != 0 && sweep && gramShortfall == nil {
			// Take the over-funding back out before the sweep so the swept amount is the one the wallet
			// really has. With a shortfall the wallet has already spent into the over-funding and there
			// is nothing to take back: leave it in and let dropOverfundFromBalances and the realBalance
			// override keep it out of the response.
			if err := deductGramBalance(emulationState, sourceAddr.ID, emulationOverfund); err != nil {
				return fail(toError(http.StatusInternalServerError, err))
			}
			emulationOverfund = 0
		}
		msgType := tonwallet.V5MsgTypeSignedExternal
		if batch.sponsored {
			msgType = tonwallet.V5MsgTypeSignedInternal
		}
		unsignedMsg, err := sourceWallet.CreateMsgBodyWithoutSignature(tonwallet.MessageConfig{
			Seqno: seqno, ValidUntil: validUntil, V5MsgType: msgType,
		}, batch.messages)
		if err != nil {
			return fail(toError(http.StatusInternalServerError, err))
		}
		var init *tlb.StateInit
		if seqno == 0 {
			init = deployInit
		}
		emuMsg, err := h.buildWalletMsgForEmulation(sourceWallet, unsignedMsg, init, batch, sourceAddr)
		if err != nil {
			return fail(toError(http.StatusInternalServerError, err))
		}
		var trace *core.Trace
		trace, emulationState, emuTime, err = h.emulateWalletMessage(ctx, emuMsg, emulationState, emuTime)
		if err != nil {
			if errors.Is(err, errEmulationFailed) {
				return fail(toError(http.StatusConflict, err))
			}
			return fail(toProperEmulationError(err))
		}
		if err := traceError(trace, seqno); err != nil {
			return fail(toError(http.StatusConflict, err))
		}
		if emulationOverfund != 0 {
			// The sweep spends the whole balance, over-funding included, so its delta measures nothing.
			if !sweep {
				gramSpent, err := gramBalanceSpent(emulationState, sourceAddr.ID, emulationStartBalance)
				if err != nil {
					return fail(toError(http.StatusInternalServerError, err))
				}
				gramShortfall = worseShortfall(gramShortfall, gramSpent, int64(gramBalance))
			}
			dropOverfundFromBalances(trace, sourceAddr.ID, emulationOverfund)
		}

		transaction, err := h.buildEmulatedTrace(ctx, trace, sourceAddr, batch, destAddr, int64(gramBalance), currency, todayRates, currencyPtr, unsignedMsg, seqno, init)
		if err != nil {
			return fail(toError(http.StatusInternalServerError, err))
		}
		resp.Transactions = append(resp.Transactions, transaction)
		seqno += 1
	}
	if gramShortfall != nil {
		// Note: the sweep was emulated on over-funded money, so raw message values inside the trace are
		// inflated. dropOverfundFromBalances corrects the end balances, and buildEmulatedTrace pins the
		// sweep's risk and TON transfer amount to the real balance — that is what clients render.
		return migrationConflict(resp, *gramShortfall), nil
	}
	return resp, nil
}

// prepareMigrationPlan builds the chunked batch plan. gasJettonMaster, when set, is moved to the last sponsored slot
// and the relay commission is priced and embedded into every sponsored batch.
func (h *Handler) prepareMigrationPlan(ctx context.Context, sourceAddr ton.Address, destAddr ton.Address, relayFunded bool, gramBalance tlb.Grams, sourceWallet *tonwallet.Wallet, gasJettonMaster *ton.AccountID) (migrationPlan, error) {
	nftTransfers, err := h.prepareNFTTransfers(ctx, sourceAddr.ID, destAddr.ID)
	if err != nil {
		return nil, err
	}
	jettons, err := h.collectMigratableJettons(ctx, sourceAddr.ID)
	if err != nil {
		return nil, err
	}
	if len(nftTransfers) == 0 && len(jettons) == 0 {
		gasJettonMaster = nil // nothing to sponsor — plain (possibly sweep-only) plan
	}
	buildPlan := func(gasReserve *big.Int) ([]migrationBatch, error) {
		jettonTransfers, err := prepareJettonTransfers(sourceAddr.ID, destAddr.ID, jettons, gasJettonMaster, gasReserve)
		if err != nil {
			return nil, err
		}
		plan, err := assembleMigrationPlan(destAddr.ID, relayFunded, gramBalance, nftTransfers, jettonTransfers)
		if err != nil {
			return nil, err
		}
		return chunkMigrationPlan(sourceWallet, relayFunded, gasJettonMaster != nil, plan), nil
	}
	var gasReserve *big.Int
	if gasJettonMaster != nil {
		// Battery's estimate emulates the batch plus a 1-nano commission placeholder, and its
		// emulator hard-fails on any jetton overdraft — a full-balance gas transfer can never
		// price. Keep one indivisible unit back for the first pricing pass; embedGaslessCommission
		// replaces the reserve with the real commission budget anyway.
		gasReserve = big.NewInt(1)
	}
	plan, err := buildPlan(gasReserve)
	if err != nil {
		return nil, err
	}
	if gasJettonMaster == nil {
		return plan, nil
	}
	plan, err = h.embedGaslessCommission(ctx, plan, buildPlan, sourceAddr.ID, sourceWallet.GetPublicKey(), *gasJettonMaster)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	return plan, nil
}

func (h *Handler) buildEmulatedTrace(ctx context.Context, trace *core.Trace, sourceAddr ton.Address, batch migrationBatch, destAddr ton.Address, realBalance int64, currency string, todayRates map[string]float64, currencyPtr *string, unsignedMsg *boc.Cell, seqno uint32, init *tlb.StateInit) (oas.MigrationTransaction, error) {
	convertedTrace := h.convertTrace(trace, h.addressBook)
	actions, err := bath.FindActions(ctx, trace, bath.ForAccount(sourceAddr.ID), bath.WithInformationSource(h.storage), bath.WithAddressBook(h.addressBook))
	if err != nil {
		return oas.MigrationTransaction{}, toError(http.StatusInternalServerError, err)
	}
	enriched := bath.EnrichWithIntentions(trace, actions)
	risk, err := wallet.ExtractRiskFromRawMessages(batch.messages)
	if err != nil {
		return oas.MigrationTransaction{}, toError(http.StatusInternalServerError, err)
	}
	if risk.TransferAllRemainingBalance {
		for _, a := range enriched.Actions {
			if a.TonTransfer != nil && a.TonTransfer.Sender == sourceAddr.ID && a.TonTransfer.Recipient == destAddr.ID {
				a.TonTransfer.Amount = realBalance
			}
		}
	}
	event, err := h.toAccountEvent(ctx, sourceAddr.ID, trace, enriched, oas.OptString{}, true)
	if err != nil {
		return oas.MigrationTransaction{}, toError(http.StatusInternalServerError, err)
	}
	enrichPreviewsWithFiat(&event, currency, todayRates)
	oasRisk, err := h.convertRisk(ctx, *risk, sourceAddr.ID, currencyPtr)
	if err != nil {
		return oas.MigrationTransaction{}, toError(http.StatusInternalServerError, err)
	}
	// convertRisk reports only the gas attached to the other messages for a mode-128 sweep. Align
	// the reported TON/Gram amount with the swept balance shown in the preview above. The fiat
	// equivalent (oasRisk.TotalEquivalent) is already computed from that balance by convertRisk.
	if oasRisk.TransferAllRemainingBalance && realBalance > oasRisk.Gram {
		oasRisk.Ton = oas.NewOptInt64(realBalance)
		oasRisk.Gram = realBalance
	}
	outMessages, err := utils.MapSliceErr(batch.messages, convertWalletMessage)
	if err != nil {
		return oas.MigrationTransaction{}, toError(http.StatusInternalServerError, err)
	}
	return convertTransaction(unsignedMsg, seqno, batch, outMessages, convertedTrace, event, oasRisk, init, traceFees(trace))
}

func (h *Handler) buildWalletMsgForEmulation(sourceWallet *tonwallet.Wallet, unsignedMsg *boc.Cell, init *tlb.StateInit, batch migrationBatch, sourceAddr ton.Address) (tlb.Message, error) {
	// A zero signature is enough for emulation: WithIgnoreSignatureDepth skips verification,
	// but the contract still expects the signature bits to be present in the body.
	signedBody, err := sourceWallet.AttachSignature(unsignedMsg, tlb.Bits512{})
	if err != nil {
		return tlb.Message{}, nil
	}
	var emuMsg tlb.Message
	if batch.sponsored {
		emuMsg, err = relayerMessage(batch, emuMsg, err, sourceAddr, signedBody, init)
	} else {
		emuMsg, err = tongo.CreateExternalMessage(sourceAddr.ID, signedBody, init, tlb.VarUInteger16{})
	}
	if err != nil {
		return tlb.Message{}, nil
	}
	return emuMsg, err
}

func relayerMessage(batch migrationBatch, emuMsg tlb.Message, err error, sourceAddr ton.Address, signedBody *boc.Cell, init *tlb.StateInit) (tlb.Message, error) {
	// Emulate what the relay would deliver: an internal message carrying the signed body,
	// with enough TON attached to fund the batch (per-transfer gas plus a fee margin).
	attach := int64(len(batch.messages)+1) * int64(migrationGasPerTransfer)
	emuMsg, _, err = tonwallet.Message{
		Amount:  tlb.Grams(attach),
		Address: sourceAddr.ID,
		Src:     &ton.AccountID{}, // mocked relay address
		Body:    signedBody,
		Bounce:  true,
		Init:    init,
	}.ToInternal()
	return emuMsg, err
}

func chunkMigrationPlan(w *tonwallet.Wallet, relayFunded, reserveCommissionSlot bool, plan []migrationBatch) (chunkedPlan []migrationBatch) {
	reserved := 0
	if reserveCommissionSlot {
		reserved = 1
	}
	chunkSize := w.MaxMessageNumber() - reserved
	if relayFunded {
		chunkSize = min(sponsoredBatchMaxMessages-reserved, chunkSize)
	}
	for _, batch := range plan {
		chunkedPlan = append(chunkedPlan, batch.chunk(chunkSize)...)
	}
	return chunkedPlan
}

func assembleMigrationPlan(to ton.AccountID, relayFunded bool, gramBalance tlb.Grams, nftTransfers, jettonTransfers []tonwallet.RawMessage) (migrationPlan, error) {
	var plan []migrationBatch

	if len(nftTransfers) > 0 {
		plan = append(plan, migrationBatch{messages: nftTransfers, sponsored: relayFunded})
	}
	if len(jettonTransfers) > 0 {
		plan = append(plan, migrationBatch{messages: jettonTransfers, sponsored: relayFunded})
	}

	// The final message sweeps the remaining TON balance to the destination.
	// The sweep is a separate, final transaction: jetton/NFT transfers generate excess messages (and bounces on failure)
	// To collect it, a separate transaction is needed
	if gramBalance > minGramTransferFee {
		sweepTransfer, err := tonwallet.ToRawMessage(tonwallet.Message{
			Amount:  0,
			Address: to,
			Bounce:  false,
			Mode:    migrationSweepMode,
		})
		if err != nil {
			return nil, err
		}
		plan = append(plan, migrationBatch{messages: []tonwallet.RawMessage{sweepTransfer}})
	}

	return plan, nil
}

func convertWalletMessage(msg tonwallet.RawMessage) (oas.MigrationOutMessage, error) {
	msgBoc, err := msg.Message.ToBocBase64()
	if err != nil {
		return oas.MigrationOutMessage{}, err
	}
	return oas.MigrationOutMessage{
		Boc:  msgBoc,
		Mode: int32(msg.Mode),
	}, nil
}

func convertTransaction(unsignedMsg *boc.Cell, seqno uint32, batch migrationBatch, outMessages []oas.MigrationOutMessage, convertedTrace oas.Trace, event oas.AccountEvent, oasRisk oas.Risk, init *tlb.StateInit, gasSpent int64) (oas.MigrationTransaction, error) {
	bocBase64, err := unsignedMsg.ToBocBase64()
	if err != nil {
		return oas.MigrationTransaction{}, err
	}
	transaction := oas.MigrationTransaction{
		Seqno:     int32(seqno),
		Boc:       bocBase64,
		Sponsored: oas.NewOptBool(batch.sponsored),
		Messages:  outMessages,
		Emulation: oas.MessageConsequences{
			Trace: convertedTrace,
			Event: event,
			Risk:  oasRisk,
		},
		GasSpent: gasSpent,
	}
	if batch.commission != nil {
		transaction.Commission = oas.NewOptString(batch.commission.String())
	}
	if init != nil {
		transaction.StateInit, err = convertStateInit(*init)
		if err != nil {
			return oas.MigrationTransaction{}, err
		}
	}
	return transaction, nil
}

// enrichPreviewsWithFiat sets the fiat equivalent of each TON or jetton transfer on its
// simple_preview fiat_value field, e.g. "99.50 USD". Best-effort: actions without a known
// market rate (e.g. NFT transfers) or currency rate are left unchanged. The rates map is keyed
// by upper-cased currency codes and by jetton raw master addresses, all denominated against a
// common TON base (same as convertRisk uses); TON itself has an implicit rate of 1.
func enrichPreviewsWithFiat(event *oas.AccountEvent, currency string, todayRates map[string]float64) {
	if len(todayRates) == 0 {
		return
	}
	curPrice, ok := todayRates[strings.ToUpper(currency)]
	if !ok || curPrice == 0 {
		return
	}
	for i := range event.Actions {
		switch {
		case event.Actions[i].TonTransfer.Set:
			// TON is the rate base, so its rate against the common base is 1.
			fiat := float64(event.Actions[i].TonTransfer.Value.Amount) / 1e9 / curPrice
			event.Actions[i].SimplePreview.FiatValue = oas.NewOptString(fmt.Sprintf("%.2f %s", fiat, strings.ToUpper(currency)))
		case event.Actions[i].JettonTransfer.Set:
			jt := event.Actions[i].JettonTransfer.Value
			rate, ok := todayRates[jt.Jetton.Address]
			if !ok || rate == 0 {
				continue
			}
			amount, ok := new(big.Float).SetString(jt.Amount) // raw indivisible units
			if !ok {
				continue
			}
			human := new(big.Float).Quo(amount, big.NewFloat(math.Pow10(jt.Jetton.Decimals)))
			fiat, _ := new(big.Float).Quo(new(big.Float).Mul(human, big.NewFloat(rate)), big.NewFloat(curPrice)).Float64()
			event.Actions[i].SimplePreview.FiatValue = oas.NewOptString(fmt.Sprintf("%.2f %s", fiat, strings.ToUpper(currency)))
		}
	}
}

type migratableJetton struct {
	wallet  core.JettonWallet
	balance oas.JettonBalance
}

// collectMigratableJettons lists the wallet's jettons eligible for migration, skipping
// locked, empty and blacklisted balances.
func (h *Handler) collectMigratableJettons(ctx context.Context, from ton.AccountID) ([]migratableJetton, error) {
	jettons, err := h.collectJettonWallets(ctx, []ton.AccountID{from})
	if err != nil {
		return nil, err
	}
	return h.getJettonMigrations(ctx, jettons[from], nil), nil
}

func (h *Handler) getJettonMigrations(ctx context.Context, wallets []core.JettonWallet, currencies []string) []migratableJetton {
	out := make([]migratableJetton, 0, len(wallets))
	for _, jw := range wallets {
		balance, err := getJettonMigrationBalance(jw, func(w core.JettonWallet) (oas.JettonBalance, error) {
			return h.convertJettonBalance(ctx, w, currencies, nil, nil)
		})
		if err != nil {
			h.logger.Warn(fmt.Sprintf("skip jetton %v: %v", jw.JettonAddress.ToRaw(), err))
			continue
		}
		out = append(out, migratableJetton{wallet: jw, balance: balance})
	}
	return out
}

// prepareJettonTransfers builds a full-balance transfer message for every jetton. Excesses
// are sent to the destination (response_destination = to). When gasJettonMaster is set,
// that jetton's transfer goes last with gasReserve kept back for the relay commission
// (see embedGaslessCommission).
func prepareJettonTransfers(from, to ton.AccountID, jettons []migratableJetton, gasJettonMaster *ton.AccountID, gasReserve *big.Int) ([]tonwallet.RawMessage, error) {
	var messages []tonwallet.RawMessage
	var gasTransfer *tonwallet.RawMessage
	for _, mj := range jettons {
		amount := mj.wallet.Balance.BigInt()
		if gasJettonMaster != nil && mj.wallet.JettonAddress == *gasJettonMaster {
			if gasReserve != nil {
				amount = amount.Sub(amount, gasReserve)
			}
			if amount.Sign() <= 0 {
				return nil, errGasJettonBalanceTooLow
			}
			transfer, err := jettonTransferRawMessage(from, to, mj.wallet.Address, amount)
			if err != nil {
				return nil, err
			}
			gasTransfer = &transfer
			continue
		}
		msgRaw, err := jettonTransferRawMessage(from, to, mj.wallet.Address, amount)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msgRaw)
	}
	if gasJettonMaster != nil {
		if gasTransfer == nil {
			return nil, toError(http.StatusBadRequest, fmt.Errorf("the source wallet holds no %v jetton", gasJettonMaster.ToRaw()))
		}
		messages = append(messages, *gasTransfer)
	}
	return messages, nil
}

func jettonTransferRawMessage(from, to, senderJettonWallet ton.AccountID, amount *big.Int) (tonwallet.RawMessage, error) {
	return tonwallet.ToRawMessage(jetton.TransferMessage{
		Sender:              from,
		SenderJettonWallet:  &senderJettonWallet,
		JettonAmount:        amount,
		Destination:         to,
		ResponseDestination: &to,
		AttachedGram:        migrationGasPerTransfer,
		ForwardGramAmount:   migrationForwardAmount,
	})
}

// requireGasJetton parses the requested commission jetton and checks that the gasless
// relay supports it. Whether the wallet holds it is checked in prepareMigrationPlan.
func (h *Handler) requireGasJetton(ctx context.Context, requested oas.OptString) (*ton.AccountID, error) {
	if !requested.IsSet() || requested.Value == "" {
		return nil, fmt.Errorf("gas_jetton_master is required when gas_payer is gasless")
	}
	master, err := tongo.ParseAddress(requested.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid gas_jetton_master: %w", err)
	}
	config, err := h.gasless.Config(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, fmt.Errorf("failed to get gasless config"))
	}
	supported := slices.ContainsFunc(config.SupportedJettons, func(s string) bool {
		id, err := ton.ParseAccountID(s)
		return err == nil && id == master.ID
	})
	if !supported {
		return nil, fmt.Errorf("jetton %v is not supported as a gasless gas jetton", master.ID.ToRaw())
	}
	return &master.ID, nil
}

// errGasJettonBalanceTooLow reports that the candidate gas jetton cannot cover the relay
// commission on top of what the migration itself transfers.
var errGasJettonBalanceTooLow = errors.New("gas jetton balance does not cover the relay commission")

func isGaslessJettonBalanceError(err error) bool {
	var extended ErrorWithExtendedCode
	if errors.As(err, &extended) {
		return extended.ExtendedCode == references.ErrGaslessNotEnoughJettons
	}
	return false
}

// embedGaslessCommission prices the relay commission via the battery Estimate handshake and
// appends the commission transfer — built by battery, so mintless payloads and attach
// amounts are correct by construction — to every sponsored batch. Pricing estimates the
// first sponsored batch, then re-runs buildPlan reserving one commission per sponsored
// batch out of the gas jetton's own transfer, and re-estimates until the price stops
// growing (it shouldn't grow when the amount shrinks). Only the first batch's commission
// is exact: later batches are re-prepared before signing (see the client contract in
// docs/migration-gasless-design.md).
func (h *Handler) embedGaslessCommission(ctx context.Context, plan []migrationBatch, buildPlan func(gasReserve *big.Int) ([]migrationBatch, error), owner ton.AccountID, pubkey ed25519.PublicKey, master ton.AccountID) ([]migrationBatch, error) {
	var sponsored []int
	for i, b := range plan {
		if b.sponsored {
			sponsored = append(sponsored, i)
		}
	}
	if len(sponsored) == 0 {
		return plan, nil
	}
	commission := new(big.Int)
	var estimated gasless.SignRawParams
	for attempt := 0; ; attempt++ {
		messages, err := g.MapSliceErr(plan[sponsored[0]].messages, func(m tonwallet.RawMessage) (string, error) {
			return m.Message.ToBocHex()
		})
		if err != nil {
			return nil, err
		}
		estimated, err = h.gasless.Estimate(ctx, gasless.EstimationParams{
			MasterID:                     master,
			WalletAddress:                owner,
			WalletPublicKey:              pubkey,
			Messages:                     messages,
			ThrowErrorIfNotEnoughJettons: attempt > 0,
		})
		if err != nil {
			if isGaslessJettonBalanceError(err) {
				return nil, fmt.Errorf("%w: %v", errGasJettonBalanceTooLow, err)
			}
			return nil, err
		}
		newCommission, ok := new(big.Int).SetString(estimated.Commission, 10)
		if !ok || newCommission.Sign() < 0 {
			return nil, fmt.Errorf("invalid commission %q in gasless estimate", estimated.Commission)
		}
		if attempt > 0 && newCommission.Cmp(commission) <= 0 {
			commission = newCommission
			break
		}
		if attempt >= 2 {
			return nil, fmt.Errorf("gasless commission estimation did not converge")
		}
		commission = newCommission
		plan, err = buildPlan(new(big.Int).Mul(commission, big.NewInt(int64(len(sponsored)))))
		if err != nil {
			return nil, err
		}
	}
	if len(estimated.Messages) != len(plan[sponsored[0]].messages)+1 {
		return nil, fmt.Errorf("gasless estimate returned %d messages for %d", len(estimated.Messages), len(plan[sponsored[0]].messages))
	}
	commissionTransfer, err := estimated.Messages[len(estimated.Messages)-1].ToWalletMessage()
	if err != nil {
		return nil, fmt.Errorf("bad commission transfer in gasless estimate: %w", err)
	}
	for _, i := range sponsored {
		plan[i].messages = append(slices.Clone(plan[i].messages), commissionTransfer)
		plan[i].commission = commission
	}
	return plan, nil
}

func (h *Handler) isWhitelistedNft(ctx context.Context, item core.NftItem, itemScam core.TrustType) bool {
	switch h.convertNFT(ctx, item, h.addressBook, h.metaCache, itemScam).Trust {
	case oas.TrustType(core.TrustWhitelist):
		return true
	default:
		return false
	}
}

// prepareNFTTransfers builds a transfer message for every migratable NFT owned by the wallet,
// skipping blacklisted items, skipped collections, and non-transferable items (e.g. SBTs). Excesses
// are sent to the destination (response_destination = to).
func (h *Handler) prepareNFTTransfers(ctx context.Context, from, to ton.AccountID) ([]tonwallet.RawMessage, error) {
	nfts, err := h.collectOwnedNFTs(ctx, from)
	if err != nil && !errors.Is(err, core.ErrEntityNotFound) {
		return nil, err
	}
	nftItemIDs := make([]ton.AccountID, 0, len(nfts))
	for _, item := range nfts {
		nftItemIDs = append(nftItemIDs, item.Address)
	}
	nftScam, scamErr := h.spamFilter.GetNftsScamData(ctx, nftItemIDs)
	if scamErr != nil {
		h.logger.Warn("error getting nft scam data", zap.Error(scamErr))
	}
	var messages []tonwallet.RawMessage
	for _, item := range nfts {
		if item.OwnerAddress == nil || *item.OwnerAddress != from {
			continue
		}
		if !h.isWhitelistedNft(ctx, item, nftScam[item.Address]) {
			continue
		}
		if isSkippedNftCollection(item.CollectionAddress) {
			continue
		}
		if !item.Transferable {
			continue
		}
		msgRaw, err := tonwallet.ToRawMessage(nft.ItemTransferMessage{
			ItemAddress:         item.Address,
			Destination:         to,
			ResponseDestination: to,
			AttachedGram:        migrationGasPerTransfer,
			ForwardGram:         migrationForwardAmount,
		})
		if err != nil {
			return nil, err
		}
		messages = append(messages, msgRaw)
	}
	return messages, nil
}

func (h *Handler) collectOwnedNFTs(ctx context.Context, owner ton.AccountID) ([]core.NftItem, error) {
	var ids []ton.AccountID
	for offset := 0; ; offset += migrationNftPageSize {
		page, err := h.storage.SearchNFTs(ctx,
			nil, // any collection
			&core.Filter[tongo.AccountID]{Value: owner},
			false, // includeOnSale: NFTs escrowed by a sale contract can't be swept by the wallet
			true,  // onlyVerified
			migrationNftPageSize,
			offset,
		)
		if err != nil {
			return nil, err
		}
		ids = append(ids, page...)
		if len(page) < migrationNftPageSize {
			break
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return h.storage.GetNFTs(ctx, ids)
}

func (h *Handler) collectJettonWallets(ctx context.Context, owners []ton.AccountID) (map[ton.AccountID][]core.JettonWallet, error) {
	wallets, err := h.storage.GetJettonWalletsByOwnerAddresses(ctx, owners, false)
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

// emulateWalletMessage emulates msg with the given account overrides. startTime (epoch, 0 = emulator default) sets the emulation clock;
// callers emulating several dependent batches must
// keep it monotonic across batches (see PrepareMigration) or storage-phase checks fail.
func (h *Handler) emulateWalletMessage(
	ctx context.Context, msg tlb.Message, overrides map[ton.AccountID]tlb.ShardAccount, startTime int64,
) (*core.Trace, map[ton.AccountID]tlb.ShardAccount, int64, error) {

	configBase64, err := h.storage.TrimmedConfigBase64()
	if err != nil {
		return nil, nil, startTime, err
	}
	options := []txemulator.TraceOption{
		txemulator.WithConfigBase64(configBase64),
		txemulator.WithAccountsSource(h.storage),
		txemulator.WithLimit(1100),
		txemulator.WithIgnoreSignatureDepth(1),
	}
	if startTime > 0 {
		options = append(options, txemulator.WithTime(startTime))
	}
	if len(overrides) > 0 {
		options = append(options, txemulator.WithAccountsMap(overrides))
	}
	emulator, err := txemulator.NewTraceBuilder(options...)
	if err != nil {
		return nil, nil, startTime, err
	}
	tree, err := emulator.Run(ctx, msg)
	if err != nil {
		return nil, nil, startTime, fmt.Errorf("%w: %w", errEmulationFailed, err)
	}
	trace, err := EmulatedTreeToTrace(ctx, h.executor, h.storage, tree, emulator.FinalStates(), nil, h.configPool, true)
	if err != nil {
		return nil, nil, startTime, err
	}
	finalStates := emulator.FinalStates()
	// The emulator runs the message tree round by round and moves its own clock forward between rounds,
	// so accounts touched late in the tree carry the latest LastPaid. Take the maximum: comparing
	// against startTime instead would make the result depend on map iteration order, and returning
	// anything below the clock the emulation actually ended on would rewind time for the next batch.
	endTime := startTime
	for _, state := range finalStates {
		if state.Account.SumType != "Account" {
			continue
		}
		endTime = max(endTime, int64(state.Account.Account.StorageStat.LastPaid)+1)
	}
	return trace, finalStates, endTime, nil
}

// resolveWallet build standard wallet, if it's not initialized it is inferred based on public key and known wallets
func resolveWallet(account tlb.ShardAccount, from ton.AccountID, publicKey oas.OptString) (*tonwallet.Wallet, uint32, *tlb.StateInit, error) {
	stateInit := account.Account.Account.Storage.State.AccountActive.StateInit
	if !(stateInit.Data.Exists && stateInit.Code.Exists) {
		pubkey, err := requirePublicKey(publicKey)
		if err != nil {
			return nil, 0, nil, toError(http.StatusBadRequest, fmt.Errorf("source wallet is not initialized; public key parameter is required then: %v", err))
		}
		w, err := tonwallet.NewFromAddress(pubkey, from)
		if err != nil {
			return nil, 0, nil, toError(http.StatusInternalServerError, fmt.Errorf("can't determine source wallet version: %w", err))
		}
		deployInit, err := w.StateInit()
		if err != nil {
			return nil, 0, nil, toError(http.StatusInternalServerError, err)
		}
		return &w, 0, deployInit, nil
	}
	w, seqno, err := tonwallet.NewFromCodeAndData(stateInit.Code.Value.Value, stateInit.Data.Value.Value, tonwallet.WithWorkchain(int(from.Workchain)))
	if err != nil {
		return nil, 0, nil, toError(http.StatusBadRequest, fmt.Errorf("unsupported source wallet: %w", err))
	}
	return &w, seqno, nil, nil
}

func getJettonMigrationBalance(jw core.JettonWallet, conv jettonConverter) (oas.JettonBalance, error) {
	if jw.Lock != nil || jw.Balance.IsZero() {
		return oas.JettonBalance{}, errJettonNotAvailableForMigration
	}
	balance, err := conv(jw)
	if err != nil {
		return oas.JettonBalance{}, err
	}
	if balance.Jetton.Verification != oas.JettonVerificationTypeWhitelist {
		return oas.JettonBalance{}, errJettonNotAvailableForMigration
	}
	return balance, nil
}

func hasMigratableAssets(wallet oas.MigrationWalletValue) bool {
	return wallet.Balance > minGramTransferFee || len(wallet.Jettons) != 0 || wallet.NftCount != 0
}

// migrationInsufficientGramMsg is the message of both 409 flavours: with the prepared plan
// (migrationConflict) and without it (errInsufficientGramForGas).
const migrationInsufficientGramMsg = "insufficient GRAM for gas"

func errInsufficientGramForGas(required, available int64) error {
	return toError(http.StatusConflict, ErrorWithExtendedCode{
		Code:         http.StatusConflict,
		Message:      migrationInsufficientGramMsg,
		ExtendedCode: references.ErrInsufficientTONForGas,
		Details: &InsufficientFunds{
			Required:  required,
			Available: available,
		},
	})
}

// migrationConflict returns the prepared migration together with the shortfall that keeps it from
// being executable, as HTTP 409 with error_code 50000. The transactions are the ones the migration
// would consist of once the source holds Required, so a client that only lacks gas can show the whole
// preview next to the shortfall instead of re-requesting prepare after the top-up. See
// MigrationPrepareConflict in api/openapi.yml.
func migrationConflict(resp *oas.MigrationPrepareResponse, shortfall InsufficientFunds) *oas.MigrationPrepareConflict {
	return &oas.MigrationPrepareConflict{
		From:          resp.From,
		To:            resp.To,
		WalletVersion: resp.WalletVersion,
		Transactions:  resp.Transactions,
		Error:         migrationInsufficientGramMsg,
		ErrorCode:     extendedCode(references.ErrInsufficientTONForGas),
		Details: oas.NewOptInsufficientFunds(oas.InsufficientFunds{
			Required:  shortfall.Required,
			Available: shortfall.Available,
		}),
	}
}

// worseShortfall folds an observed (required, available) pair into the shortfall found so far, keeping
// the larger requirement: `required` is what the source must hold for the whole plan to go through, so
// reporting the smaller of two observations would only buy the client a second failed attempt. A
// covered observation never clears a shortfall already found.
func worseShortfall(current *InsufficientFunds, required, available int64) *InsufficientFunds {
	if required <= available || (current != nil && current.Required >= required) {
		return current
	}
	return &InsufficientFunds{Required: required, Available: available}
}

// migrationValidUntil is the deadline of the batch that starts on emulation clock emuTime. emuTime is
// never behind the wall clock — it starts at time.Now() and only moves forward — so the first batch
// gets the full lifetime from now and each later one gets the full lifetime from the point its
// predecessors have landed.
func migrationValidUntil(emuTime int64) time.Time {
	return time.Unix(emuTime, 0).Add(migrationMsgLifetime)
}

func emulatedGramBalance(state map[ton.AccountID]tlb.ShardAccount, key ton.AccountID) (tlb.Grams, error) {
	shardAcc, ok := state[key]
	if !ok {
		return 0, fmt.Errorf("emulation returned no state for source %v", key.ToRaw())
	}
	if shardAcc.Account.SumType != "Account" {
		return 0, fmt.Errorf("source %v is not initialized after emulation", key.ToRaw())
	}
	return shardAcc.Account.Account.Storage.Balance.Grams, nil
}

func gramBalanceSpent(state map[ton.AccountID]tlb.ShardAccount, key ton.AccountID, startBalance tlb.Grams) (int64, error) {
	gramBalance, err := emulatedGramBalance(state, key)
	if err != nil {
		return 0, err
	}
	return int64(startBalance) - int64(gramBalance), nil
}

func deductGramBalance(state map[ton.AccountID]tlb.ShardAccount, key ton.AccountID, val tlb.Grams) error {
	gramBalance, err := emulatedGramBalance(state, key)
	if err != nil {
		return err
	}
	if gramBalance < val {
		return fmt.Errorf("source %v spent into its over-funding: balance %v is below %v", key.ToRaw(), gramBalance, val)
	}
	shardAcc := state[key]
	shardAcc.Account.Account.Storage.Balance.Grams = gramBalance - val
	state[key] = shardAcc
	return nil
}

func dropOverfundFromBalances(trace *core.Trace, key ton.AccountID, overfund tlb.Grams) {
	if trace == nil {
		return
	}
	if trace.Account == key {
		trace.EndBalance = max(trace.EndBalance-int64(overfund), 0)
	}
	for _, child := range trace.Children {
		dropOverfundFromBalances(child, key, overfund)
	}
}

// traceFees sums the TON burned as fees by every transaction in trace. The per-transaction arithmetic
// mirrors core.convertTransaction (pkg/core/converters.go) and bath.fromTrace: total_fees already
// contains the action phase's total_action_fees, i.e. only the validator's cut of each outbound
// message's forward fee, so swapping that for the full total_fwd_fees also accounts for the part
// collected downstream when the message is imported. Emulating against an over-funded source balance
// does not distort this: gas is instruction count, forward fees are cell and bit count, storage fees
// are cell count and elapsed time — none of them depend on the balance.
func traceFees(trace *core.Trace) int64 {
	if trace == nil {
		return 0
	}
	fees := trace.TotalFee
	if phase := trace.ActionPhase; phase != nil {
		fees += int64(phase.FwdFees) - int64(phase.TotalFees)
	}
	for _, child := range trace.Children {
		fees += traceFees(child)
	}
	return fees
}

func traceError(trace *core.Trace, seqno uint32) error {
	if trace == nil {
		return fmt.Errorf("%w on seqno %v: emulation returned no trace", errEmulationFailed, seqno)
	}
	if trace.Success && !trace.Aborted {
		return nil
	}
	var reasons []error
	if phase := trace.ComputePhase; phase != nil {
		switch {
		case phase.Skipped:
			reasons = append(reasons, fmt.Errorf("compute phase skipped: %v", phase.SkipReason))
		case !phase.Success:
			reasons = append(reasons, fmt.Errorf("compute exit code %v", phase.ExitCode))
		}
	}
	if phase := trace.ActionPhase; phase != nil && !phase.Success {
		reasons = append(reasons, fmt.Errorf("action result code %v", phase.ResultCode))
	}
	if len(reasons) == 0 {
		reasons = append(reasons, fmt.Errorf("aborted=%v success=%v", trace.Aborted, trace.Success))
	}
	return fmt.Errorf("%w on seqno %v: %w", errEmulationFailed, seqno, errors.Join(reasons...))
}
