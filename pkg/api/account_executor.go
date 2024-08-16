package api

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/tvm"
	"github.com/tonkeeper/tongo/tvm/precompiled"

	"github.com/tonkeeper/opentonapi/pkg/core"
)

var precompileUsageMc = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "emulation_precompile_usage",
}, []string{"status", "hash"})

type shardsAccountExecutor struct {
	accounts   map[tongo.AccountID]tlb.ShardAccount
	resolver   core.LibraryResolver
	executor   executor
	configPool *sync.Pool
}

func newSharedAccountExecutor(accounts map[tongo.AccountID]tlb.ShardAccount, executor executor, resolver core.LibraryResolver, configPool *sync.Pool) *shardsAccountExecutor {
	return &shardsAccountExecutor{
		accounts:   accounts,
		resolver:   resolver,
		executor:   executor,
		configPool: configPool,
	}
}

func (s shardsAccountExecutor) RunSmcMethodByID(ctx context.Context, accountID tongo.AccountID, methodID int, params tlb.VmStack) (uint32, tlb.VmStack, error) {
	account, ok := s.accounts[accountID]
	if !ok {
		return s.executor.RunSmcMethodByID(ctx, accountID, methodID, params)
	}
	code, data := accountCode(account), accountData(account)
	if code == nil || data == nil {
		return 0, nil, errors.New("account not found")
	}
	codeHash, err := code.Hash()
	if err != nil {
		return 0, nil, err
	}
	precompile := precompiled.KnownMethods[precompiled.MethodCode{MethodID: methodID, CodeHash: [32]byte(codeHash)}]
	if precompile != nil {
		stack, err := precompile(data, params)
		if err == nil {
			precompileUsageMc.WithLabelValues("success", hex.EncodeToString(codeHash)).Inc()
			return 0, stack, nil
		}
		precompileUsageMc.WithLabelValues("unsuccess", hex.EncodeToString(codeHash)).Inc()
	} else {
		precompileUsageMc.WithLabelValues("not_found", hex.EncodeToString(codeHash)).Inc()
	}
	codeBoc, err := code.ToBocBase64()
	if err != nil {
		return 0, nil, err
	}
	dataBoc, err := data.ToBocBase64()
	if err != nil {
		return 0, nil, err
	}
	libraries := core.StateInitLibraries(accountLibraries(account))
	librariesBase64, err := core.PrepareLibraries(ctx, code, libraries, s.resolver)
	if err != nil {
		return 0, nil, err
	}
	configObject := s.configPool.Get().(*tvm.Config)
	defer s.configPool.Put(configObject)

	if configObject == nil {
		return 0, nil, errors.New("error getting BlockchainConfig from the pool")
	}

	e, err := tvm.NewEmulatorFromBOCsBase64(codeBoc, dataBoc, "",
		tvm.WithLibrariesBase64(librariesBase64),
		tvm.WithLibraryResolver(s.resolver),
		tvm.WithConfig(configObject))
	if err != nil {
		return 0, nil, err
	}
	return e.RunSmcMethodByID(ctx, accountID, methodID, params)
}

func accountCode(account tlb.ShardAccount) *boc.Cell {
	if account.Account.SumType == "AccountNone" {
		return nil
	}
	if account.Account.Account.Storage.State.SumType != "AccountActive" {
		return nil
	}
	code := account.Account.Account.Storage.State.AccountActive.StateInit.Code
	if !code.Exists {
		return nil
	}
	cell := code.Value.Value
	return &cell
}

func accountData(account tlb.ShardAccount) *boc.Cell {
	if account.Account.SumType == "AccountNone" {
		return nil
	}
	if account.Account.Account.Storage.State.SumType != "AccountActive" {
		return nil
	}
	data := account.Account.Account.Storage.State.AccountActive.StateInit.Data
	if !data.Exists {
		return nil
	}
	cell := data.Value.Value
	return &cell
}

func accountLibraries(account tlb.ShardAccount) *tlb.HashmapE[tlb.Bits256, tlb.SimpleLib] {
	if account.Account.SumType == "AccountNone" {
		return nil
	}
	if account.Account.Account.Storage.State.SumType != "AccountActive" {
		return nil
	}
	return &account.Account.Account.Storage.State.AccountActive.StateInit.Library
}
