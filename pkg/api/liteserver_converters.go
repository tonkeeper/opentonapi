package api

import (
	"encoding/hex"

	"github.com/tonkeeper/opentonapi/pkg/oas"
	"github.com/tonkeeper/tongo/liteclient"
)

func toJson(target *string, source interface{ MarshalJSON() ([]byte, error) }) error {
	bytes, err := source.MarshalJSON()
	*target = hex.EncodeToString(bytes)
	return err
}

func errChain(mappings ...error) error {
	for i := range mappings {
		if mappings[i] != nil {
			return mappings[i]
		}
	}
	return nil
}

func convertMasterchainInfo(info liteclient.LiteServerMasterchainInfoC) (*oas.GetMasterchainInfoLiteServerOK, error) {
	convertedInfo := oas.GetMasterchainInfoLiteServerOK{
		Last: oas.BlockRaw{
			Workchain: info.Last.Workchain,
			Shard:     info.Last.Shard,
			Seqno:     info.Last.Seqno,
		},
		Init: oas.InitStateRaw{
			Workchain: info.Init.Workchain,
		},
	}
	err := errChain(
		toJson(&convertedInfo.Last.RootHash, info.Last.RootHash),
		toJson(&convertedInfo.Last.FileHash, info.Last.FileHash),
		toJson(&convertedInfo.StateRootHash, info.StateRootHash),
		toJson(&convertedInfo.Init.RootHash, info.Init.RootHash),
		toJson(&convertedInfo.Init.FileHash, info.Init.FileHash),
	)
	if err != nil {
		return nil, err
	}
	return &convertedInfo, nil
}

func convertMasterchainInfoExt(info liteclient.LiteServerMasterchainInfoExtC) (*oas.GetMasterchainInfoExtLiteServerOK, error) {
	convertedInfo := oas.GetMasterchainInfoExtLiteServerOK{
		Mode:         info.Mode,
		Version:      info.Version,
		Capabilities: info.Capabilities,
		Last: oas.BlockRaw{
			Workchain: info.Last.Workchain,
			Shard:     info.Last.Shard,
			Seqno:     info.Last.Seqno,
		},
		LastUtime: info.LastUtime,
		Now:       info.Now,
		Init: oas.InitStateRaw{
			Workchain: info.Init.Workchain,
		},
	}
	err := errChain(
		toJson(&convertedInfo.Last.RootHash, info.Last.RootHash),
		toJson(&convertedInfo.Last.FileHash, info.Last.FileHash),
		toJson(&convertedInfo.StateRootHash, info.StateRootHash),
		toJson(&convertedInfo.Init.RootHash, info.Init.RootHash),
		toJson(&convertedInfo.Init.FileHash, info.Init.FileHash),
	)
	if err != nil {
		return nil, err
	}
	return &convertedInfo, nil
}

func convertBlock(block liteclient.LiteServerBlockDataC) (*oas.GetBlockLiteServerOK, error) {
	convertedBlock := oas.GetBlockLiteServerOK{
		ID: oas.BlockRaw{
			Workchain: block.Id.Workchain,
			Shard:     block.Id.Shard,
			Seqno:     block.Id.Seqno,
		},
		Data: hex.EncodeToString(block.Data),
	}
	err := errChain(
		toJson(&convertedBlock.ID.RootHash, block.Id.RootHash),
		toJson(&convertedBlock.ID.FileHash, block.Id.FileHash),
	)
	if err != nil {
		return nil, err
	}
	return &convertedBlock, nil
}

func convertState(state liteclient.LiteServerBlockStateC) (*oas.GetStateLiteServerOK, error) {
	convertedState := oas.GetStateLiteServerOK{
		ID: oas.BlockRaw{
			Workchain: state.Id.Workchain,
			Shard:     state.Id.Shard,
			Seqno:     state.Id.Seqno,
		},
		Data: hex.EncodeToString(state.Data),
	}
	err := errChain(
		toJson(&convertedState.ID.RootHash, state.Id.RootHash),
		toJson(&convertedState.ID.FileHash, state.Id.FileHash),
		toJson(&convertedState.RootHash, state.RootHash),
		toJson(&convertedState.FileHash, state.FileHash),
	)
	if err != nil {
		return nil, err
	}
	return &convertedState, nil
}

func convertBlockHeaderRaw(blockHeader liteclient.LiteServerBlockHeaderC) (*oas.GetBlockHeaderLiteServerOK, error) {
	convertedBlockHeader := oas.GetBlockHeaderLiteServerOK{
		ID: oas.BlockRaw{
			Workchain: blockHeader.Id.Workchain,
			Shard:     blockHeader.Id.Shard,
			Seqno:     blockHeader.Id.Seqno,
		},
		Mode:        blockHeader.Mode,
		HeaderProof: hex.EncodeToString(blockHeader.HeaderProof),
	}
	err := errChain(
		toJson(&convertedBlockHeader.ID.RootHash, blockHeader.Id.RootHash),
		toJson(&convertedBlockHeader.ID.FileHash, blockHeader.Id.FileHash),
	)
	if err != nil {
		return nil, err
	}
	return &convertedBlockHeader, nil
}

func convertAccountState(accountState liteclient.LiteServerAccountStateC) (*oas.GetAccountStateLiteServerOK, error) {
	convertedAccountState := oas.GetAccountStateLiteServerOK{
		ID: oas.BlockRaw{
			Workchain: accountState.Id.Workchain,
			Shard:     accountState.Id.Shard,
			Seqno:     accountState.Id.Seqno,
		},
		Shardblk: oas.BlockRaw{
			Workchain: accountState.Shardblk.Workchain,
			Shard:     accountState.Shardblk.Shard,
			Seqno:     accountState.Shardblk.Seqno,
		},
		ShardProof: hex.EncodeToString(accountState.ShardProof),
		Proof:      hex.EncodeToString(accountState.Proof),
		State:      hex.EncodeToString(accountState.State),
	}
	err := errChain(
		toJson(&convertedAccountState.ID.RootHash, accountState.Id.RootHash),
		toJson(&convertedAccountState.ID.FileHash, accountState.Id.FileHash),
		toJson(&convertedAccountState.Shardblk.RootHash, accountState.Shardblk.RootHash),
		toJson(&convertedAccountState.Shardblk.FileHash, accountState.Shardblk.FileHash),
	)
	if err != nil {
		return nil, err
	}
	return &convertedAccountState, nil
}

func convertShardInfo(shardInfo liteclient.LiteServerShardInfoC) (*oas.GetShardInfoLiteServerOK, error) {
	convertedShardInfo := oas.GetShardInfoLiteServerOK{
		ID: oas.BlockRaw{
			Workchain: shardInfo.Id.Workchain,
			Shard:     shardInfo.Id.Shard,
			Seqno:     shardInfo.Id.Seqno,
		},
		Shardblk: oas.BlockRaw{
			Workchain: shardInfo.Shardblk.Workchain,
			Shard:     shardInfo.Shardblk.Shard,
			Seqno:     shardInfo.Shardblk.Seqno,
		},
		ShardProof: hex.EncodeToString(shardInfo.ShardProof),
		ShardDescr: hex.EncodeToString(shardInfo.ShardDescr),
	}
	err := errChain(
		toJson(&convertedShardInfo.ID.RootHash, shardInfo.Id.RootHash),
		toJson(&convertedShardInfo.ID.FileHash, shardInfo.Id.FileHash),
		toJson(&convertedShardInfo.Shardblk.RootHash, shardInfo.Shardblk.RootHash),
		toJson(&convertedShardInfo.Shardblk.FileHash, shardInfo.Shardblk.FileHash),
	)
	if err != nil {
		return nil, err
	}
	return &convertedShardInfo, nil
}

func convertShardsAllInfo(shardsAllInfo liteclient.LiteServerAllShardsInfoC) (*oas.GetAllShardsInfoLiteServerOK, error) {
	convertedShardsAllInfo := oas.GetAllShardsInfoLiteServerOK{
		ID: oas.BlockRaw{
			Workchain: shardsAllInfo.Id.Workchain,
			Shard:     shardsAllInfo.Id.Shard,
			Seqno:     shardsAllInfo.Id.Seqno,
		},
		Proof: hex.EncodeToString(shardsAllInfo.Proof),
		Data:  hex.EncodeToString(shardsAllInfo.Data),
	}
	err := errChain(
		toJson(&convertedShardsAllInfo.ID.RootHash, shardsAllInfo.Id.RootHash),
		toJson(&convertedShardsAllInfo.ID.FileHash, shardsAllInfo.Id.FileHash),
	)
	if err != nil {
		return nil, err
	}
	return &convertedShardsAllInfo, nil
}

func convertTransactions(txs liteclient.LiteServerTransactionListC) (*oas.GetTransactionsLiteServerOK, error) {
	convertedTransactions := oas.GetTransactionsLiteServerOK{
		Transactions: hex.EncodeToString(txs.Transactions),
	}
	for _, tx := range txs.Ids {
		blockRaw := oas.BlockRaw{
			Workchain: tx.Workchain,
			Shard:     tx.Shard,
			Seqno:     tx.Seqno,
		}
		err := errChain(
			toJson(&blockRaw.RootHash, tx.RootHash),
			toJson(&blockRaw.FileHash, tx.FileHash),
		)
		if err != nil {
			return nil, err
		}
		convertedTransactions.Ids = append(convertedTransactions.Ids, blockRaw)
	}
	return &convertedTransactions, nil
}

func convertListBlockTxs(txs liteclient.LiteServerBlockTransactionsC) (*oas.GetListBlockTransactionsLiteServerOK, error) {
	convertedTxs := &oas.GetListBlockTransactionsLiteServerOK{
		ID: oas.BlockRaw{
			Workchain: txs.Id.Workchain,
			Shard:     txs.Id.Shard,
			Seqno:     txs.Id.Seqno,
		},
		ReqCount:   txs.ReqCount,
		Incomplete: txs.Incomplete,
		Proof:      hex.EncodeToString(txs.Proof),
	}
	err := errChain(
		toJson(&convertedTxs.ID.RootHash, txs.Id.RootHash),
		toJson(&convertedTxs.ID.FileHash, txs.Id.FileHash),
	)
	if err != nil {
		return nil, err
	}
	for _, tx := range txs.Ids {
		item := oas.GetListBlockTransactionsLiteServerOKIdsItem{Mode: tx.Mode}
		if tx.Account != nil {
			err = errChain(toJson(&item.Account.Value, tx.Account))
			if err != nil {
				return nil, err
			}
		}
		if tx.Lt != nil {
			item.Lt.Value = *tx.Lt
		}
		if tx.Hash != nil {
			err = errChain(toJson(&item.Hash.Value, tx.Hash))
			if err != nil {
				return nil, err
			}
		}
		convertedTxs.Ids = append(convertedTxs.Ids, item)
	}
	return convertedTxs, nil
}

func convertBlockProof(blockProof liteclient.LiteServerPartialBlockProofC) (*oas.GetBlockProofLiteServerOK, error) {
	convertedBlockProof := &oas.GetBlockProofLiteServerOK{
		Complete: blockProof.Complete,
		From: oas.BlockRaw{
			Workchain: blockProof.From.Workchain,
			Shard:     blockProof.From.Shard,
			Seqno:     blockProof.From.Seqno,
		},
		To: oas.BlockRaw{
			Workchain: blockProof.To.Workchain,
			Shard:     blockProof.To.Shard,
			Seqno:     blockProof.To.Seqno,
		},
	}
	err := errChain(
		toJson(&convertedBlockProof.From.RootHash, blockProof.From.RootHash),
		toJson(&convertedBlockProof.From.FileHash, blockProof.From.FileHash),
		toJson(&convertedBlockProof.To.RootHash, blockProof.To.RootHash),
		toJson(&convertedBlockProof.To.FileHash, blockProof.To.FileHash),
	)
	if err != nil {
		return nil, err
	}
	for _, step := range blockProof.Steps {
		signatures := []oas.GetBlockProofLiteServerOKStepsItemLiteServerBlockLinkForwardSignaturesSignaturesItem{}
		for _, signature := range step.LiteServerBlockLinkForward.Signatures.Signatures {
			item := oas.GetBlockProofLiteServerOKStepsItemLiteServerBlockLinkForwardSignaturesSignaturesItem{
				Signature: hex.EncodeToString(signature.Signature),
			}
			err = errChain(toJson(&item.NodeIDShort, signature.NodeIdShort))
			if err != nil {
				return nil, err
			}
			signatures = append(signatures, item)
		}
		item := oas.GetBlockProofLiteServerOKStepsItem{
			LiteServerBlockLinkBack: oas.GetBlockProofLiteServerOKStepsItemLiteServerBlockLinkBack{
				ToKeyBlock: step.LiteServerBlockLinkBack.ToKeyBlock,
				From: oas.BlockRaw{
					Workchain: step.LiteServerBlockLinkBack.From.Workchain,
					Shard:     step.LiteServerBlockLinkBack.From.Shard,
					Seqno:     step.LiteServerBlockLinkBack.From.Seqno,
				},
				To: oas.BlockRaw{
					Workchain: step.LiteServerBlockLinkBack.To.Workchain,
					Shard:     step.LiteServerBlockLinkBack.To.Shard,
					Seqno:     step.LiteServerBlockLinkBack.To.Seqno,
				},
				DestProof:  hex.EncodeToString(step.LiteServerBlockLinkBack.DestProof),
				Proof:      hex.EncodeToString(step.LiteServerBlockLinkBack.Proof),
				StateProof: hex.EncodeToString(step.LiteServerBlockLinkBack.StateProof),
			},
			LiteServerBlockLinkForward: oas.GetBlockProofLiteServerOKStepsItemLiteServerBlockLinkForward{
				ToKeyBlock: step.LiteServerBlockLinkForward.ToKeyBlock,
				From: oas.BlockRaw{
					Workchain: step.LiteServerBlockLinkForward.From.Workchain,
					Shard:     step.LiteServerBlockLinkForward.From.Shard,
					Seqno:     step.LiteServerBlockLinkForward.From.Seqno,
				},
				To: oas.BlockRaw{
					Workchain: step.LiteServerBlockLinkForward.To.Workchain,
					Shard:     step.LiteServerBlockLinkForward.To.Shard,
					Seqno:     step.LiteServerBlockLinkForward.To.Seqno,
				},
				DestProof:   hex.EncodeToString(step.LiteServerBlockLinkForward.DestProof),
				ConfigProof: hex.EncodeToString(step.LiteServerBlockLinkForward.ConfigProof),
				Signatures: oas.GetBlockProofLiteServerOKStepsItemLiteServerBlockLinkForwardSignatures{
					ValidatorSetHash: step.LiteServerBlockLinkForward.Signatures.ValidatorSetHash,
					CatchainSeqno:    step.LiteServerBlockLinkForward.Signatures.CatchainSeqno,
					Signatures:       signatures,
				},
			},
		}
		err = errChain(
			toJson(&item.LiteServerBlockLinkBack.From.RootHash, step.LiteServerBlockLinkBack.From.RootHash),
			toJson(&item.LiteServerBlockLinkBack.From.FileHash, step.LiteServerBlockLinkBack.From.FileHash),
			toJson(&item.LiteServerBlockLinkBack.To.RootHash, step.LiteServerBlockLinkBack.To.RootHash),
			toJson(&item.LiteServerBlockLinkBack.To.FileHash, step.LiteServerBlockLinkBack.To.FileHash),
			toJson(&item.LiteServerBlockLinkForward.From.RootHash, step.LiteServerBlockLinkForward.From.RootHash),
			toJson(&item.LiteServerBlockLinkForward.From.FileHash, step.LiteServerBlockLinkForward.From.FileHash),
			toJson(&item.LiteServerBlockLinkForward.To.RootHash, step.LiteServerBlockLinkForward.To.RootHash),
			toJson(&item.LiteServerBlockLinkForward.To.FileHash, step.LiteServerBlockLinkForward.To.FileHash),
		)
		if err != nil {
			return nil, err
		}
		convertedBlockProof.Steps = append(convertedBlockProof.Steps, item)
	}
	return convertedBlockProof, nil
}

func convertConfig(config liteclient.LiteServerConfigInfoC) (*oas.GetConfigAllLiteServerOK, error) {
	convertedConfig := oas.GetConfigAllLiteServerOK{
		Mode: config.Mode,
		ID: oas.BlockRaw{
			Workchain: config.Id.Workchain,
			Shard:     config.Id.Shard,
			Seqno:     config.Id.Seqno,
		},
		StateProof:  hex.EncodeToString(config.StateProof),
		ConfigProof: hex.EncodeToString(config.ConfigProof),
	}
	err := errChain(
		toJson(&convertedConfig.ID.RootHash, config.Id.RootHash),
		toJson(&convertedConfig.ID.FileHash, config.Id.FileHash),
	)
	if err != nil {
		return nil, err
	}
	return &convertedConfig, nil
}

func convertShardBlockProof(shardBlockProof liteclient.LiteServerShardBlockProofC) (*oas.GetShardBlockProofLiteServerOK, error) {
	convertedShardBlockProof := oas.GetShardBlockProofLiteServerOK{
		MasterchainID: oas.BlockRaw{
			Workchain: shardBlockProof.MasterchainId.Workchain,
			Shard:     shardBlockProof.MasterchainId.Shard,
			Seqno:     shardBlockProof.MasterchainId.Seqno,
		},
	}
	err := errChain(
		toJson(&convertedShardBlockProof.MasterchainID.RootHash, shardBlockProof.MasterchainId.RootHash),
		toJson(&convertedShardBlockProof.MasterchainID.FileHash, shardBlockProof.MasterchainId.FileHash),
	)
	if err != nil {
		return nil, err
	}
	for _, link := range shardBlockProof.Links {
		shardBlock := oas.GetShardBlockProofLiteServerOKLinksItem{
			ID: oas.BlockRaw{
				Workchain: link.Id.Workchain,
				Shard:     link.Id.Shard,
				Seqno:     link.Id.Seqno,
			},
			Proof: hex.EncodeToString(link.Proof),
		}
		err = errChain(
			toJson(&shardBlock.ID.RootHash, link.Id.RootHash),
			toJson(&shardBlock.ID.FileHash, link.Id.FileHash),
		)
		if err != nil {
			return nil, err
		}
		convertedShardBlockProof.Links = append(convertedShardBlockProof.Links, shardBlock)
	}
	return &convertedShardBlockProof, nil
}
