package api

import (
	"encoding/hex"
	"fmt"

	"github.com/arnac-io/opentonapi/pkg/oas"
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

func convertMasterchainInfo(info liteclient.LiteServerMasterchainInfoC) (*oas.GetRawMasterchainInfoOK, error) {
	convertedInfo := oas.GetRawMasterchainInfoOK{
		Last: oas.BlockRaw{
			Workchain: int32(info.Last.Workchain),
			Shard:     fmt.Sprintf("%016x", info.Last.Shard),
			Seqno:     int32(info.Last.Seqno),
		},
		Init: oas.InitStateRaw{
			Workchain: int32(info.Init.Workchain),
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

func convertMasterchainInfoExt(info liteclient.LiteServerMasterchainInfoExtC) (*oas.GetRawMasterchainInfoExtOK, error) {
	convertedInfo := oas.GetRawMasterchainInfoExtOK{
		Mode:         int32(info.Mode),
		Version:      int32(info.Version),
		Capabilities: int64(info.Capabilities),
		Last: oas.BlockRaw{
			Workchain: int32(info.Last.Workchain),
			Shard:     fmt.Sprintf("%016x", info.Last.Shard),
			Seqno:     int32(info.Last.Seqno),
		},
		LastUtime: int32(info.LastUtime),
		Now:       int32(info.Now),
		Init: oas.InitStateRaw{
			Workchain: int32(info.Init.Workchain),
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

func convertBlock(block liteclient.LiteServerBlockDataC) (*oas.GetRawBlockchainBlockOK, error) {
	convertedBlock := oas.GetRawBlockchainBlockOK{
		ID:   convertBlockIDRaw(block.Id),
		Data: hex.EncodeToString(block.Data),
	}
	return &convertedBlock, nil
}

func convertState(state liteclient.LiteServerBlockStateC) (*oas.GetRawBlockchainBlockStateOK, error) {
	convertedState := oas.GetRawBlockchainBlockStateOK{
		ID:       convertBlockIDRaw(state.Id),
		Data:     hex.EncodeToString(state.Data),
		RootHash: hex.EncodeToString(state.RootHash[:]),
		FileHash: hex.EncodeToString(state.FileHash[:]),
	}
	return &convertedState, nil
}

func convertBlockHeaderRaw(blockHeader liteclient.LiteServerBlockHeaderC) (*oas.GetRawBlockchainBlockHeaderOK, error) {
	convertedBlockHeader := oas.GetRawBlockchainBlockHeaderOK{
		ID:          convertBlockIDRaw(blockHeader.Id),
		Mode:        int32(blockHeader.Mode),
		HeaderProof: hex.EncodeToString(blockHeader.HeaderProof),
	}
	return &convertedBlockHeader, nil
}

func convertBlockIDRaw(id liteclient.TonNodeBlockIdExtC) oas.BlockRaw {
	return oas.BlockRaw{
		Workchain: int32(id.Workchain),
		Shard:     fmt.Sprintf("%016x", id.Shard),
		Seqno:     int32(id.Seqno),
		RootHash:  fmt.Sprintf("%x", id.RootHash[:]),
		FileHash:  fmt.Sprintf("%x", id.FileHash[:]),
	}
}

func convertAccountState(accountState liteclient.LiteServerAccountStateC) (*oas.GetRawAccountStateOK, error) {
	convertedAccountState := oas.GetRawAccountStateOK{
		ID:         convertBlockIDRaw(accountState.Id),
		Shardblk:   convertBlockIDRaw(accountState.Shardblk),
		ShardProof: hex.EncodeToString(accountState.ShardProof),
		Proof:      hex.EncodeToString(accountState.Proof),
		State:      hex.EncodeToString(accountState.State),
	}
	return &convertedAccountState, nil
}

func convertShardInfo(shardInfo liteclient.LiteServerShardInfoC) (*oas.GetRawShardInfoOK, error) {
	convertedShardInfo := oas.GetRawShardInfoOK{
		ID:         convertBlockIDRaw(shardInfo.Id),
		Shardblk:   convertBlockIDRaw(shardInfo.Shardblk),
		ShardProof: hex.EncodeToString(shardInfo.ShardProof),
		ShardDescr: hex.EncodeToString(shardInfo.ShardDescr),
	}
	return &convertedShardInfo, nil
}

func convertShardsAllInfo(shardsAllInfo liteclient.LiteServerAllShardsInfoC) (*oas.GetAllRawShardsInfoOK, error) {
	convertedShardsAllInfo := oas.GetAllRawShardsInfoOK{
		ID:    convertBlockIDRaw(shardsAllInfo.Id),
		Proof: hex.EncodeToString(shardsAllInfo.Proof),
		Data:  hex.EncodeToString(shardsAllInfo.Data),
	}
	return &convertedShardsAllInfo, nil
}

func convertTransactions(txs liteclient.LiteServerTransactionListC) (*oas.GetRawTransactionsOK, error) {
	convertedTransactions := oas.GetRawTransactionsOK{
		Transactions: hex.EncodeToString(txs.Transactions),
	}
	for _, tx := range txs.Ids {
		blockRaw := convertBlockIDRaw(tx)
		convertedTransactions.Ids = append(convertedTransactions.Ids, blockRaw)
	}
	return &convertedTransactions, nil
}

func convertListBlockTxs(txs liteclient.LiteServerBlockTransactionsC) (*oas.GetRawListBlockTransactionsOK, error) {
	convertedTxs := &oas.GetRawListBlockTransactionsOK{
		ID:         convertBlockIDRaw(txs.Id),
		ReqCount:   int32(txs.ReqCount),
		Incomplete: txs.Incomplete,
		Proof:      hex.EncodeToString(txs.Proof),
	}
	for _, tx := range txs.Ids {
		item := oas.GetRawListBlockTransactionsOKIdsItem{Mode: int32(tx.Mode)}
		if tx.Account != nil {
			err := errChain(toJson(&item.Account.Value, tx.Account))
			if err != nil {
				return nil, err
			}
		}
		if tx.Lt != nil {
			item.Lt.Value = int64(*tx.Lt)
		}
		if tx.Hash != nil {
			err := errChain(toJson(&item.Hash.Value, tx.Hash))
			if err != nil {
				return nil, err
			}
		}
		convertedTxs.Ids = append(convertedTxs.Ids, item)
	}
	return convertedTxs, nil
}

func convertBlockProof(blockProof liteclient.LiteServerPartialBlockProofC) (*oas.GetRawBlockProofOK, error) {
	convertedBlockProof := &oas.GetRawBlockProofOK{
		Complete: blockProof.Complete,
		From:     convertBlockIDRaw(blockProof.From),
		To:       convertBlockIDRaw(blockProof.To),
	}
	for _, step := range blockProof.Steps {
		signatures := []oas.GetRawBlockProofOKStepsItemLiteServerBlockLinkForwardSignaturesSignaturesItem{}
		for _, signature := range step.LiteServerBlockLinkForward.Signatures.Signatures {
			item := oas.GetRawBlockProofOKStepsItemLiteServerBlockLinkForwardSignaturesSignaturesItem{
				Signature: hex.EncodeToString(signature.Signature),
			}
			err := errChain(toJson(&item.NodeIDShort, signature.NodeIdShort))
			if err != nil {
				return nil, err
			}
			signatures = append(signatures, item)
		}
		item := oas.GetRawBlockProofOKStepsItem{
			LiteServerBlockLinkBack: oas.GetRawBlockProofOKStepsItemLiteServerBlockLinkBack{
				ToKeyBlock: step.LiteServerBlockLinkBack.ToKeyBlock,
				From:       convertBlockIDRaw(step.LiteServerBlockLinkBack.From),
				To:         convertBlockIDRaw(step.LiteServerBlockLinkBack.To),
				DestProof:  hex.EncodeToString(step.LiteServerBlockLinkBack.DestProof),
				Proof:      hex.EncodeToString(step.LiteServerBlockLinkBack.Proof),
				StateProof: hex.EncodeToString(step.LiteServerBlockLinkBack.StateProof),
			},
			LiteServerBlockLinkForward: oas.GetRawBlockProofOKStepsItemLiteServerBlockLinkForward{
				ToKeyBlock:  step.LiteServerBlockLinkForward.ToKeyBlock,
				From:        convertBlockIDRaw(step.LiteServerBlockLinkForward.From),
				To:          convertBlockIDRaw(step.LiteServerBlockLinkForward.To),
				DestProof:   hex.EncodeToString(step.LiteServerBlockLinkForward.DestProof),
				ConfigProof: hex.EncodeToString(step.LiteServerBlockLinkForward.ConfigProof),
				Signatures: oas.GetRawBlockProofOKStepsItemLiteServerBlockLinkForwardSignatures{
					ValidatorSetHash: int64(step.LiteServerBlockLinkForward.Signatures.ValidatorSetHash),
					CatchainSeqno:    int32(step.LiteServerBlockLinkForward.Signatures.CatchainSeqno),
					Signatures:       signatures,
				},
			},
		}
		convertedBlockProof.Steps = append(convertedBlockProof.Steps, item)
	}
	return convertedBlockProof, nil
}

func convertRawConfig(config liteclient.LiteServerConfigInfoC) (*oas.GetRawConfigOK, error) {
	convertedConfig := oas.GetRawConfigOK{
		Mode:        int32(config.Mode),
		ID:          convertBlockIDRaw(config.Id),
		StateProof:  hex.EncodeToString(config.StateProof),
		ConfigProof: hex.EncodeToString(config.ConfigProof),
	}
	return &convertedConfig, nil
}

func convertShardBlockProof(shardBlockProof liteclient.LiteServerShardBlockProofC) (*oas.GetRawShardBlockProofOK, error) {
	convertedShardBlockProof := oas.GetRawShardBlockProofOK{
		MasterchainID: convertBlockIDRaw(shardBlockProof.MasterchainId),
	}
	for _, link := range shardBlockProof.Links {
		shardBlock := oas.GetRawShardBlockProofOKLinksItem{
			ID:    convertBlockIDRaw(link.Id),
			Proof: hex.EncodeToString(link.Proof),
		}
		convertedShardBlockProof.Links = append(convertedShardBlockProof.Links, shardBlock)
	}
	return &convertedShardBlockProof, nil
}
