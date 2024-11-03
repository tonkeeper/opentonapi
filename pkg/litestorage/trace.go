package litestorage

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/ton"

	"github.com/arnac-io/opentonapi/pkg/core"
)

// Taken from https://github.com/tonkeeper/tongo/blob/master/abi/interfaces.go#L1018
var knownContracts = map[ton.Bits256]abi.ContractInterface{
	ton.MustParseHash("0dceed21269d66013e95b19fbb5c55a6f01adad40837baa8e521cde3a02aa46c"): abi.WalletHighloadV1R2,
	ton.MustParseHash("11acad7955844090f283bf238bc1449871f783e7cc0979408d3f4859483e8525"): abi.WalletHighloadV3R1,
	ton.MustParseHash("1bd9c5a39bffb7a0f341588b5dd92b813a842bf65ef14109382200ceaf8f72df"): abi.NftAuctionGetgemsV3,
	ton.MustParseHash("203dd4f358adb49993129aa925cac39916b68a0e4f78d26e8f2c2b69eafa5679"): abi.WalletHighloadV2R2,
	ton.MustParseHash("20834b7b72b112147e1b2fb457b84e74d1a30f04f737d4f62a668e9552d2b72f"): abi.WalletV5R1,
	ton.MustParseHash("24221fa571e542e055c77bedfdbf527c7af460cfdc7f344c450787b4cfa1eb4d"): abi.NftSaleGetgemsV3,
	ton.MustParseHash("32050dfac44f64866bcc86f2cd9e1305fe9dcadb3959c002237cfb0902d44323"): abi.NftSaleGetgemsV3,
	ton.MustParseHash("45ebbce9b5d235886cb6bfe1c3ad93b708de058244892365c9ee0dfe439cb7b5"): abi.WalletPreprocessedV2,
	ton.MustParseHash("4c9123828682fa6f43797ab41732bca890cae01766e0674100250516e0bf8d42"): abi.NftItemSimple,
	ton.MustParseHash("587cc789eff1c84f46ec3797e45fc809a14ff5ae24f1e0c7a6a99cc9dc9061ff"): abi.WalletV1R3,
	ton.MustParseHash("5c9a5e68c108e18721a07c42f9956bfb39ad77ec6d624b60c576ec88eee65329"): abi.WalletV2R1,
	ton.MustParseHash("64dd54805522c5be8a9db59cea0105ccf0d08786ca79beb8cb79e880a8d7322d"): abi.WalletV4R1,
	ton.MustParseHash("6668872fa79705443ffd47523e8e9ea9f76ab99f9a0b59d27de8f81a1c27b9d4"): abi.NftAuctionGetgemsV3,
	ton.MustParseHash("8278f4c5233de6fbedc969af519344a7a9bffc544856dba986a95c0bcf8571c9"): abi.NftSaleGetgemsV2,
	ton.MustParseHash("84dafa449f98a6987789ba232358072bc0f76dc4524002a5d0918b9a75d2d599"): abi.WalletV3R2,
	ton.MustParseHash("89468f02c78e570802e39979c8516fc38df07ea76a48357e0536f2ba7b3ee37b"): abi.JettonWalletGoverned,
	ton.MustParseHash("8ceb45b3cd4b5cc60eaae1c13b9c092392677fe536b2e9b2d801b62eff931fe1"): abi.WalletHighloadV2R1,
	ton.MustParseHash("8d28ea421b77e805fea52acf335296499f03aec8e9fd21ddb5f2564aa65c48de"): abi.JettonWalletV2,
	ton.MustParseHash("9494d1cc8edf12f05671a1a9ba09921096eb50811e1924ec65c3c629fbb80812"): abi.WalletHighloadV2,
	ton.MustParseHash("a01e057fbd4288402b9898d78d67bd4e90254c93c5866879bc2d1d12865436bc"): abi.MultisigOrderV2,
	ton.MustParseHash("a0cfc2c48aee16a271f2cfc0b7382d81756cecb1017d077faaab3bb602f6868c"): abi.WalletV1R1,
	ton.MustParseHash("b61041a58a7980b946e8fb9e198e3c904d24799ffa36574ea4251c41a566f581"): abi.WalletV3R1,
	ton.MustParseHash("beb0683ebeb8927fe9fc8ec0a18bc7dd17899689825a121eab46c5a3a860d0ce"): abi.JettonWalletV1,
	ton.MustParseHash("ccae6ffb603c7d3e779ab59ec267ffc22dc1ebe0af9839902289a7a83e4c00f1"): abi.GramMiner,
	ton.MustParseHash("d3d14da9a627f0ec3533341829762af92b9540b21bf03665fac09c2b46eabbac"): abi.MultisigV2,
	ton.MustParseHash("d4902fcc9fad74698fa8e353220a68da0dcf72e32bcb2eb9ee04217c17d3062c"): abi.WalletV1R2,
	ton.MustParseHash("d8cdbbb79f2c5caa677ac450770be0351be21e1250486de85cc52aa33dd16484"): abi.WalletHighloadV1R1,
	ton.MustParseHash("deb53b6c5765c1e6cd238bf47bc5e83ba596bdcc04b0b84cd50ab1e474a08f31"): abi.NftSaleGetgemsV3,
	ton.MustParseHash("e4cf3b2f4c6d6a61ea0f2b5447d266785b26af3637db2deee6bcd1aa826f3412"): abi.WalletV5Beta,
	ton.MustParseHash("f3d7ca53493deedac28b381986a849403cbac3d2c584779af081065af0ac4b93"): abi.WalletV5Beta,
	ton.MustParseHash("fe9530d3243853083ef2ef0b4c2908c0abf6fa1c31ea243aacaa5bf8c7d753f1"): abi.WalletV2R2,
	ton.MustParseHash("feb5ff6820e2ff0d9483e7e0d62c817d846789fb4ae580c878866d959dabd5c0"): abi.WalletV4R2,
}

const (
	maxDepthLimit = 1024
)

var (
	emulatedAccountCode = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "tonapi_emulated_account_code_litestorage_counter",
	}, []string{"code_hash"})
)

func (s *LiteStorage) GetTrace(ctx context.Context, hash tongo.Bits256) (*core.Trace, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		storageTimeHistogramVec.WithLabelValues("get_trace").Observe(v)
	}))
	defer timer.ObserveDuration()
	tx, err := s.GetTransaction(ctx, hash)
	if err != nil {
		return nil, err
	}
	s.logger.Info(fmt.Sprintf("Found transaction hash %s in cache", hash.Hex()))
	root, err := s.findRoot(ctx, tx, 0)
	if err != nil {
		return nil, err
	}
	s.logger.Info(fmt.Sprintf("Found transaction root %s", root.Hash.Hex()))
	trace, err := s.recursiveGetChildren(ctx, *root, 0)
	if err != nil {
		foundHashes := []string{}
		core.Visit(&trace, func(trace *core.Trace) {
			foundHashes = append(foundHashes, trace.Transaction.Hash.Hex())
		})

		s.logger.Info(fmt.Sprintf("Didn't find all children of transaction: %s root: %s", hash.Hex(), root.Hash.Hex()))
		s.logger.Info(fmt.Sprintf("Hashes found %s", foundHashes))
	}
	return &trace, err
}

func (s *LiteStorage) SearchTraces(ctx context.Context, a tongo.AccountID, limit int, beforeLT, startTime, endTime *int64, initiator bool) ([]core.TraceID, error) {
	return nil, nil
}

func (s *LiteStorage) recursiveGetChildren(ctx context.Context, tx core.Transaction, depth int) (core.Trace, error) {
	trace := core.Trace{Transaction: tx}
	externalMessages := make([]core.Message, 0, len(tx.OutMsgs))
	for _, m := range tx.OutMsgs {
		if m.Destination == nil {
			externalMessages = append(externalMessages, m)
			continue
		}
		tx := s.searchTxInCache(*m.Destination, m.CreatedLt)
		if tx == nil {
			return core.Trace{}, core.ErrEntityNotFound
		}
		child, err := s.recursiveGetChildren(ctx, *tx, depth+1)
		if err != nil {
			return core.Trace{}, err
		}
		trace.Children = append(trace.Children, &child)
	}
	var err error
	trace.AccountInterfaces, err = s.getAccountInterfaces(ctx, tx.Account)
	if err != nil {
		return core.Trace{}, nil
	}
	trace.OutMsgs = externalMessages
	return trace, nil
}

func (s *LiteStorage) findRoot(ctx context.Context, tx *core.Transaction, depth int) (*core.Transaction, error) {
	if tx == nil {
		return nil, fmt.Errorf("can't find root of nil transaction")
	}
	if tx.InMsg == nil || tx.InMsg.IsExternal() || tx.InMsg.IsEmission() {
		if tx.InMsg == nil {
			s.logger.Info("transaction is root InMsg is nil")
		} else {
			s.logger.Info(fmt.Sprintf("transaction is root isExternal: %t isEmission: %t", tx.InMsg.IsExternal(), tx.InMsg.IsEmission()))
		}
		return tx, nil
	}
	source := *tx.InMsg.Source
	createdLt := tx.InMsg.CreatedLt
	tx = s.searchTxInCache(source, createdLt)
	if tx == nil {
		s.logger.Info(fmt.Sprintf("Didn't find transaction parent accountId: %s, createdLt: %d", source.String(), createdLt))
		return nil, core.ErrEntityNotFound
	}
	return s.findRoot(ctx, tx, depth+1)
}

func (s *LiteStorage) searchTransactionNearBlock(ctx context.Context, a tongo.AccountID, lt uint64, blockID tongo.BlockID, back bool, depth int) (*core.Transaction, error) {
	if depth > maxDepthLimit {
		return nil, fmt.Errorf("can't find tx because of depth limit")
	}
	tx := s.searchTxInCache(a, lt)
	if tx != nil {
		return tx, nil
	}
	tx, err := s.searchTransactionInBlock(ctx, a, lt, blockID, back)
	if err != nil {
		if back {
			blockID.Seqno--
		} else {
			blockID.Seqno++
		}
		tx, err = s.searchTransactionInBlock(ctx, a, lt, blockID, back)
		if err != nil {
			return nil, err
		}
	}

	return tx, nil
}

func (s *LiteStorage) searchTransactionInBlock(ctx context.Context, a tongo.AccountID, lt uint64, blockID tongo.BlockID, back bool) (*core.Transaction, error) {
	blockIDExt, _, err := s.client.LookupBlock(ctx, blockID, 1, nil, nil)
	if err != nil {
		return nil, err
	}
	block, prs := s.blockCache.Load(blockIDExt)
	if !prs {
		b, err := s.client.GetBlock(ctx, blockIDExt)
		if err != nil {
			return nil, err
		}
		s.blockCache.Store(blockIDExt, &b)
		block = &b
	}
	for _, tx := range block.AllTransactions() {
		if tx.AccountAddr != a.Address {
			continue
		}
		inMsg := tx.Msgs.InMsg
		if !back && inMsg.Exists && inMsg.Value.Value.Info.IntMsgInfo != nil && inMsg.Value.Value.Info.IntMsgInfo.CreatedLt == lt {
			return core.ConvertTransaction(a.Workchain, tongo.Transaction{BlockID: blockIDExt, Transaction: *tx})
		}
		if back {
			for _, m := range tx.Msgs.OutMsgs.Values() {
				if m.Value.Info.IntMsgInfo != nil && m.Value.Info.IntMsgInfo.CreatedLt == lt {
					return core.ConvertTransaction(a.Workchain, tongo.Transaction{BlockID: blockIDExt, Transaction: *tx})
				}
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

func (s *LiteStorage) getAccountInterfaces(ctx context.Context, id tongo.AccountID) ([]abi.ContractInterface, error) {
	interfaces, ok := s.accountInterfacesCache.Load(id)
	if ok {
		return interfaces, nil
	}
	account, err := s.GetRawAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(account.Code) > 0 {
		cells, err := boc.DeserializeBoc(account.Code)
		if err != nil {
			return nil, err
		}
		if len(cells) == 0 {
			return nil, fmt.Errorf("failed to find a root cell")
		}
		h, err := cells[0].HashString()
		if err != nil {
			return nil, err
		}
		emulatedAccountCode.WithLabelValues(h).Inc()
	}
	contractInterface, err := InspectContract(account.Code)
	if err != nil {
		return nil, err
	}
	if contractInterface == nil {
		return nil, nil
	}
	interfaces = []abi.ContractInterface{*contractInterface}
	s.accountInterfacesCache.Store(id, interfaces)
	return interfaces, nil
}

func InspectContract(code []byte) (*abi.ContractInterface, error) {
	// Originally the code was:
	// inspector := abi.NewContractInspector(abi.InspectWithLibraryResolver(s))
	// cd, err := inspector.InspectContract(ctx, account.Code, s.executor, id)
	// That contains a more complex logic, for finding the interface using both hardcoded known contracts hashes,
	// and by manually inspecting and trying all known methods, of the contract
	// This also contained more complext logic for getting extra data that isn't really needed,
	// so we can simplify it to comparing to the known contract hashes
	if len(code) == 0 {
		return nil, nil
	}
	cells, err := boc.DeserializeBoc(code)
	if err != nil {
		return nil, err
	}
	if len(cells) == 0 {
		return nil, fmt.Errorf("failed to find a root cell")
	}
	root := cells[0]
	codeHash, err := root.Hash256()
	if err != nil {
		return nil, err
	}

	if contractInterface, ok := knownContracts[codeHash]; ok {
		return &contractInterface, nil
	}
	return nil, nil
}
