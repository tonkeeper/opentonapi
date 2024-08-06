package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/exp/maps"

	"github.com/tonkeeper/tongo/contract/elector"
	"github.com/tonkeeper/tongo/tvm"

	"github.com/tonkeeper/opentonapi/internal/g"
	"github.com/tonkeeper/opentonapi/pkg/core"
	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

func (h *Handler) Status(ctx context.Context) (*oas.ServiceStatus, error) {
	latency, seqno, err := h.storage.GetLatencyAndLastMasterchainSeqno(ctx)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	var restOnline = true
	if err != nil {
		restOnline = false
	}
	return &oas.ServiceStatus{
		IndexingLatency:           int(latency),
		RestOnline:                restOnline,
		LastKnownMasterchainSeqno: int32(seqno),
	}, nil
}

func (h *Handler) GetReducedBlockchainBlocks(ctx context.Context, params oas.GetReducedBlockchainBlocksParams) (*oas.ReducedBlocks, error) {
	if params.From > params.To {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("from must be less (or equal) than to"))
	}
	if params.To-params.From > 600 {
		return nil, toError(http.StatusBadRequest, fmt.Errorf("max diapason is 10 minutes"))
	}
	blocks, err := h.storage.GetReducedBlocks(ctx, params.From, params.To)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	var converted []oas.ReducedBlock
	for _, block := range blocks {
		converted = append(converted, convertReducedBlock(block))
	}
	return &oas.ReducedBlocks{Blocks: converted}, nil
}

func (h *Handler) GetBlockchainBlock(ctx context.Context, params oas.GetBlockchainBlockParams) (*oas.BlockchainBlock, error) {
	blockID, err := ton.ParseBlockID(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	block, err := h.storage.GetBlockHeader(ctx, blockID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	res := convertBlockHeader(*block)
	return &res, nil
}

func (h *Handler) GetBlockchainMasterchainShards(ctx context.Context, params oas.GetBlockchainMasterchainShardsParams) (r *oas.BlockchainBlockShards, _ error) {
	shards, err := h.storage.GetBlockShards(ctx, ton.BlockID{Shard: 0x8000000000000000, Seqno: uint32(params.MasterchainSeqno), Workchain: -1})
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}

	res := oas.BlockchainBlockShards{
		Shards: make([]oas.BlockchainBlockShardsShardsItem, len(shards)),
	}
	for i, shard := range shards {
		block, err := h.storage.GetBlockHeader(ctx, shard)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		res.Shards[i] = oas.BlockchainBlockShardsShardsItem{
			LastKnownBlockID: shard.String(),
			LastKnownBlock:   convertBlockHeader(*block),
		}
	}
	return &res, nil
}

func (h *Handler) blocksDiff(ctx context.Context, masterchainSeqno int32) ([]ton.BlockID, error) {
	shards, err := h.storage.GetBlockShards(ctx, ton.BlockID{Shard: 0x8000000000000000, Seqno: uint32(masterchainSeqno), Workchain: -1})
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	prevShards, err := h.storage.GetBlockShards(ctx, ton.BlockID{Shard: 0x8000000000000000, Seqno: uint32(masterchainSeqno) - 1, Workchain: -1})
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	blocks := []ton.BlockID{{Shard: 0x8000000000000000, Seqno: uint32(masterchainSeqno), Workchain: -1}}

	for _, s := range shards {
		missedBlocks, err := findMissedBlocks(ctx, h.storage, s, prevShards)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, missedBlocks...)
	}

	return blocks, nil
}

func findMissedBlocks(ctx context.Context, s storage, id ton.BlockID, prev []ton.BlockID) ([]ton.BlockID, error) {
	for _, p := range prev {
		if id.Shard == p.Shard && id.Workchain == p.Workchain {
			blocks := make([]ton.BlockID, 0, int(id.Seqno-p.Seqno))
			for i := p.Seqno + 1; i <= id.Seqno; i++ {
				blocks = append(blocks, ton.BlockID{Workchain: p.Workchain, Shard: p.Shard, Seqno: i})
			}
			return blocks, nil
		}
	}
	blocks := []ton.BlockID{id}
	header, err := s.GetBlockHeader(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, p := range header.PrevBlocks {
		missed, err := findMissedBlocks(ctx, s, p.BlockID, prev)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, missed...)
	}
	uniq := make(map[ton.BlockID]struct{}, len(blocks))
	for i := range blocks {
		uniq[blocks[i]] = struct{}{}
	}
	if len(blocks) == len(uniq) {
		return blocks, nil
	}
	return maps.Keys(uniq), nil
}

func (h *Handler) GetBlockchainMasterchainBlocks(ctx context.Context, params oas.GetBlockchainMasterchainBlocksParams) (*oas.BlockchainBlocks, error) {
	blockIDs, err := h.blocksDiff(ctx, params.MasterchainSeqno)
	if err != nil {
		return nil, err
	}
	result := oas.BlockchainBlocks{
		Blocks: make([]oas.BlockchainBlock, len(blockIDs)),
	}
	for i, id := range blockIDs {
		block, err := h.storage.GetBlockHeader(ctx, id)
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		result.Blocks[i] = convertBlockHeader(*block)
	}
	return &result, nil
}

func (h *Handler) GetBlockchainMasterchainTransactions(ctx context.Context, params oas.GetBlockchainMasterchainTransactionsParams) (*oas.Transactions, error) {
	blockIDs, err := h.blocksDiff(ctx, params.MasterchainSeqno)
	if err != nil {
		return nil, err
	}
	var result oas.Transactions
	for _, id := range blockIDs {
		txs, err := h.storage.GetBlockTransactions(ctx, id)
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		for _, tx := range txs {
			result.Transactions = append(result.Transactions, convertTransaction(*tx, nil, h.addressBook))
		}
	}
	return &result, nil
}

func (h *Handler) GetBlockchainBlockTransactions(ctx context.Context, params oas.GetBlockchainBlockTransactionsParams) (*oas.Transactions, error) {
	blockID, err := ton.ParseBlockID(params.BlockID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	transactions, err := h.storage.GetBlockTransactions(ctx, blockID)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	res := oas.Transactions{
		Transactions: make([]oas.Transaction, 0, len(transactions)),
	}
	for _, tx := range transactions {
		res.Transactions = append(res.Transactions, convertTransaction(*tx, nil, h.addressBook))
	}
	return &res, nil
}

func (h *Handler) GetBlockchainTransaction(ctx context.Context, params oas.GetBlockchainTransactionParams) (*oas.Transaction, error) {
	hash, err := tongo.ParseHash(params.TransactionID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	txs, err := h.storage.GetTransaction(ctx, hash)
	if errors.Is(err, core.ErrEntityNotFound) {
		var txHash *tongo.Bits256
		txHash, err = h.storage.SearchTransactionByMessageHash(ctx, hash)
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		txs, err = h.storage.GetTransaction(ctx, *txHash)
		if errors.Is(err, core.ErrEntityNotFound) {
			return nil, toError(http.StatusNotFound, err)
		}
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	transaction := convertTransaction(*txs, nil, h.addressBook)
	return &transaction, nil
}

func (h *Handler) GetBlockchainTransactionByMessageHash(ctx context.Context, params oas.GetBlockchainTransactionByMessageHashParams) (*oas.Transaction, error) {
	hash, err := tongo.ParseHash(params.MsgID)
	if err != nil {
		return nil, toError(http.StatusBadRequest, err)
	}
	txHash, err := h.storage.SearchTransactionByMessageHash(ctx, hash)
	if errors.Is(err, core.ErrEntityNotFound) {
		return nil, toError(http.StatusNotFound, fmt.Errorf("transaction not found"))
	} else if errors.Is(err, core.ErrTooManyEntities) {
		return nil, toError(http.StatusNotFound, fmt.Errorf("more than one transaction with messages hash"))
	} else if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	txs, err := h.storage.GetTransaction(ctx, *txHash)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	transaction := convertTransaction(*txs, nil, h.addressBook)
	return &transaction, nil
}

func (h *Handler) GetBlockchainMasterchainHead(ctx context.Context) (*oas.BlockchainBlock, error) {
	header, err := h.storage.LastMasterchainBlockHeader(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return g.Pointer(convertBlockHeader(*header)), nil
}

func (h *Handler) GetBlockchainConfig(ctx context.Context) (*oas.BlockchainConfig, error) {
	rawCfg, err := h.storage.GetConfigRaw(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	cfg, err := ton.DecodeConfigParams(rawCfg)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	c := boc.NewCell()
	err = tlb.Marshal(c, cfg) //todo: optimize. we can not marshal but take config cell directly from boc
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	raw, err := c.ToBocString()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	out, err := convertConfig(h.logger, cfg)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	out.Raw = raw
	return out, nil
}

func (h *Handler) GetBlockchainConfigFromBlock(ctx context.Context, params oas.GetBlockchainConfigFromBlockParams) (*oas.BlockchainConfig, error) {
	blockID := ton.BlockID{
		Workchain: -1,
		Shard:     0x8000000000000000,
		Seqno:     uint32(params.MasterchainSeqno),
	}
	cfg, err := h.storage.GetConfigFromBlock(ctx, blockID)
	if err != nil && errors.Is(err, core.ErrNotKeyBlock) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	c := boc.NewCell()
	err = tlb.Marshal(c, cfg)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	raw, err := c.ToBocString()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	out, err := convertConfig(h.logger, cfg)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	out.Raw = raw
	return out, nil
}

func convertConfigToOasConfig(conf *ton.BlockchainConfig) (*oas.RawBlockchainConfig, error) {
	// TODO: optimize this workflow
	value, err := json.Marshal(*conf)
	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	if err := json.Unmarshal(g.ChangeJsonKeys(value, g.CamelToSnake), &m); err != nil {
		return nil, err
	}
	return &oas.RawBlockchainConfig{Config: anyToJSONRawMap(m)}, nil
}

func (h *Handler) GetRawBlockchainConfig(ctx context.Context) (r *oas.RawBlockchainConfig, _ error) {
	config, err := h.storage.GetLastConfig(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	rawConfig, err := convertConfigToOasConfig(&config)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return rawConfig, nil
}

func (h *Handler) GetRawBlockchainConfigFromBlock(ctx context.Context, params oas.GetRawBlockchainConfigFromBlockParams) (r *oas.RawBlockchainConfig, _ error) {
	blockID := ton.BlockID{
		Workchain: -1,
		Shard:     0x8000000000000000,
		Seqno:     uint32(params.MasterchainSeqno),
	}
	cfg, err := h.storage.GetConfigFromBlock(ctx, blockID)
	if err != nil && errors.Is(err, core.ErrNotKeyBlock) {
		return nil, toError(http.StatusNotFound, err)
	}
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	config, err := ton.ConvertBlockchainConfigStrict(cfg)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	rawConfig, err := convertConfigToOasConfig(config)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	return rawConfig, nil
}

func (h *Handler) GetBlockchainValidators(ctx context.Context) (*oas.Validators, error) {
	blockHeader, err := h.storage.LastMasterchainBlockHeader(ctx)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	next := blockHeader.BlockID
	configBlockID := next
	if !blockHeader.IsKeyBlock {
		configBlockID.Seqno = uint32(blockHeader.PrevKeyBlockSeqno)
	}
	rawConfig, err := h.storage.GetConfigFromBlock(ctx, configBlockID)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	config, err := ton.ConvertBlockchainConfigStrict(rawConfig)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	bConfig, err := h.storage.TrimmedConfigBase64()
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	configCell, err := boc.DeserializeSinglRootBase64(bConfig)
	if err != nil {
		return nil, toError(http.StatusInternalServerError, err)
	}
	electorAddr, ok := config.ElectorAddr()
	if !ok {
		return nil, toError(http.StatusInternalServerError, fmt.Errorf("can't get elector address"))
	}
	iterations := 0
	for {
		iterations += 1
		if iterations > 100 {
			return nil, toError(http.StatusInternalServerError, fmt.Errorf("can't find block with elections"))
		}
		blockHeader, err := h.storage.GetBlockHeader(ctx, next)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		next.Seqno = uint32(blockHeader.PrevKeyBlockSeqno)
		if !blockHeader.IsKeyBlock {
			continue
		}
		electorState, err := h.storage.GetAccountStateRaw(ctx, electorAddr, &blockHeader.BlockIDExt)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		electorStateCell, err := boc.DeserializeBoc(electorState.State)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		var acc tlb.Account
		err = tlb.Unmarshal(electorStateCell[0], &acc)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		init := acc.Account.Storage.State.AccountActive.StateInit
		code := init.Code.Value.Value
		data := init.Data.Value.Value

		emulator, err := tvm.NewEmulator(&code, &data, configCell)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		if err := emulator.SetGasLimit(10_000_000); err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		list, err := elector.GetParticipantListExtended(ctx, electorAddr, emulator)
		if err != nil {
			return nil, toError(http.StatusInternalServerError, err)
		}
		if config.ConfigParam34 == nil {
			return nil, toError(http.StatusInternalServerError, fmt.Errorf("there is no current validators set in blockchain config"))
		}

		validatorSet := config.ConfigParam34.CurValidators
		var pubKeys map[tlb.Bits256]struct{}
		var utimeSince uint32
		switch validatorSet.SumType {
		case "Validators":
			utimeSince = validatorSet.Validators.UtimeSince
			pubKeys = make(map[tlb.Bits256]struct{}, len(validatorSet.Validators.List.Keys()))
			for _, descr := range validatorSet.Validators.List.Values() {
				pubKeys[descr.PubKey()] = struct{}{}
			}
		case "ValidatorsExt":
			utimeSince = validatorSet.ValidatorsExt.UtimeSince
			pubKeys = make(map[tlb.Bits256]struct{}, len(validatorSet.ValidatorsExt.List.Keys()))
			for _, descr := range validatorSet.ValidatorsExt.List.Values() {
				pubKeys[descr.PubKey()] = struct{}{}
			}
		default:
			return nil, toError(http.StatusInternalServerError, fmt.Errorf("unknown validator set type %v", validatorSet.SumType))
		}
		if list.ElectAt != int64(utimeSince) {
			// this election is for the next validator set,
			// let's take travel back in time to the current validator set
			continue
		}
		validators := &oas.Validators{
			ElectAt:    list.ElectAt,
			ElectClose: list.ElectClose,
			MinStake:   list.MinStake,
			TotalStake: list.TotalStake,
			Validators: make([]oas.Validator, 0, len(list.Validators)),
		}
		for _, v := range list.Validators {
			// sometimes, participant_list_extended returns validators that are not in the current validator set,
			// so we need to filter them out.
			if _, ok := pubKeys[v.Pubkey]; !ok {
				continue
			}
			validators.Validators = append(validators.Validators, oas.Validator{
				Stake:       v.Stake,
				MaxFactor:   v.MaxFactor,
				Address:     v.Address.ToRaw(),
				AdnlAddress: v.AdnlAddr,
			})
		}
		return validators, nil
	}
}
