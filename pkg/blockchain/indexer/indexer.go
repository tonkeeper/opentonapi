package indexer

import (
	"context"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/conc/iter"
	"github.com/tonkeeper/tonapi-go"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"
)

type chunk struct {
	masterID tongo.BlockID
	ids      map[tongo.BlockIDExt]struct{}
	blocks   []IDandBlock
}

// Indexer tracks the blockchain and notifies subscribers about new blocks.
type Indexer struct {
	logger       *zap.Logger
	cli          *liteapi.Client
	tonapiClient *tonapi.Client
}

func New(logger *zap.Logger, cli *liteapi.Client, tonapiClient *tonapi.Client) *Indexer {
	return &Indexer{
		cli:          cli,
		logger:       logger,
		tonapiClient: tonapiClient,
	}
}

type IDandBlock struct {
	ID             tongo.BlockIDExt
	Block          *tlb.Block
	BlockTimestamp int64
	Transactions   []tonapi.Transaction
}

func (idx *Indexer) Run(ctx context.Context, channels []chan IDandBlock) {
	var chunk *chunk
	for {
		time.Sleep(200 * time.Millisecond)
		info, err := idx.cli.GetMasterchainInfo(ctx)
		if err != nil {
			idx.logger.Error("failed to get masterchain info", zap.Error(err))
			continue
		}
		chunk, err = idx.initChunk(info.Last.Seqno)
		if err != nil {
			idx.logger.Error("failed to get init chunk", zap.Error(err))
			continue
		}
		break
	}

	for {
		time.Sleep(500 * time.Millisecond)
		next, err := idx.next(chunk)
		if err != nil {
			if isBlockNotReadyError(err) {
				continue
			}
			idx.logger.Error("failed to get next chunk", zap.Error(err))
			continue
		}
		for _, block := range next.blocks {
			for _, ch := range channels {
				ch <- block
			}
		}
		chunk = next
	}

}

func (idx *Indexer) GetBlocksFromMasterBlock(masterBlockNumber uint32) (IDandBlock, []IDandBlock, error) {
	chunk, err := idx.initChunk(masterBlockNumber)
	if err != nil {
		idx.logger.Error("failed to get init chunk", zap.Error(err))
		return IDandBlock{}, nil, err
	}
	workChainBlocks := []IDandBlock{}
	var masterBlock *IDandBlock
	for i, block := range chunk.blocks {
		if block.ID.BlockID.Workchain != -1 {
			workChainBlocks = append(workChainBlocks, block)
		} else if block.ID.BlockID.Workchain == -1 {
			if masterBlock != nil {
				return IDandBlock{}, nil, errors.New("more than one master block")
			}
			masterBlock = &chunk.blocks[i]
		}
	}
	if masterBlock == nil {
		return IDandBlock{}, nil, errors.New("no master block")
	}
	return *masterBlock, workChainBlocks, nil
}

func (idx *Indexer) next(prevChunk *chunk) (*chunk, error) {
	nextMasterID := prevChunk.masterID
	nextMasterID.Seqno += 1
	masterBlockID, _, err := idx.cli.LookupBlock(context.Background(), nextMasterID, 1, nil, nil)
	if err != nil {
		return nil, err
	}
	masterBlock, err := idx.cli.GetBlock(context.Background(), masterBlockID)
	if err != nil {
		return nil, err
	}
	shards := tongo.ShardIDs(&masterBlock)
	currentChunk := chunk{
		masterID: nextMasterID,
		ids:      make(map[tongo.BlockIDExt]struct{}, len(shards)+1),
	}
	for _, shardID := range shards {
		currentChunk.ids[shardID] = struct{}{}
	}

	var chunkBlocks []IDandBlock
	// queue contains IDs to resolve
	// and we are going to resolve each ID to a corresponding block
	queue := shards
	for {
		if len(queue) == 0 {
			break
		}
		blocks, err := iter.MapErr[tongo.BlockIDExt, *tlb.Block](queue, func(t *tongo.BlockIDExt) (*tlb.Block, error) {
			if _, ok := prevChunk.ids[*t]; ok {
				return nil, nil
			}
			block, err := idx.cli.GetBlock(context.Background(), *t)
			if err != nil {
				if strings.Contains(err.Error(), "not in db") {
					return nil, nil
				}
				return nil, err
			}
			return &block, nil
		})
		if err != nil {
			return nil, err
		}
		var newQueue []tongo.BlockIDExt
		for i, block := range blocks {
			if block == nil {
				continue
			}
			chunkBlocks = append(chunkBlocks, IDandBlock{ID: queue[i], Block: block})
			parents, err := tongo.GetParents(block.Info)
			if err != nil {
				return nil, err
			}
			for _, parent := range parents {
				if _, ok := prevChunk.ids[parent]; ok {
					continue
				}
				if _, ok := currentChunk.ids[parent]; ok {
					continue
				}
				// we need to get block of this parent and its parents
				newQueue = append(newQueue, parent)
			}
		}
		queue = newQueue
	}
	for i := len(chunkBlocks)/2 - 1; i >= 0; i-- {
		opp := len(chunkBlocks) - 1 - i
		chunkBlocks[i], chunkBlocks[opp] = chunkBlocks[opp], chunkBlocks[i]
	}
	chunkBlocks = append(chunkBlocks, IDandBlock{ID: masterBlockID, Block: &masterBlock})
	sort.Slice(chunkBlocks, func(i, j int) bool {
		return chunkBlocks[i].Block.Info.StartLt < chunkBlocks[j].Block.Info.StartLt
	})
	currentChunk.blocks = chunkBlocks
	return &currentChunk, nil
}

func (idx *Indexer) initChunk(seqno uint32) (*chunk, error) {
	init := tongo.BlockID{
		Workchain: -1,
		Shard:     9223372036854775808,
		Seqno:     seqno,
	}
	blocks, err := idx.tonapiClient.GetBlockchainMasterchainBlocks(context.Background(), tonapi.GetBlockchainMasterchainBlocksParams{int32(seqno)})
	if err != nil {
		return nil, err
	}

	ch := &chunk{
		masterID: init,
		ids:      map[tongo.BlockIDExt]struct{}{},
		blocks:   []IDandBlock{},
	}

	for _, block := range blocks.Blocks {
		shardNumber, err := strconv.ParseUint(block.Shard, 16, 64)
		if err != nil {
			return nil, err
		}
		blockID := tongo.BlockID{
			Workchain: block.WorkchainID,
			Shard:     shardNumber,
			Seqno:     uint32(block.Seqno),
		}
		rootHash := ton.Bits256{}
		bytes, err := hex.DecodeString(block.RootHash)
		if err != nil {
			return nil, err
		}
		copy(rootHash[:], bytes)

		fileHash := ton.Bits256{}
		bytes, err = hex.DecodeString(block.FileHash)
		if err != nil {
			return nil, err
		}
		copy(fileHash[:], bytes)

		id := tongo.BlockIDExt{
			BlockID:  blockID,
			RootHash: rootHash,
			FileHash: fileHash,
		}
		ch.ids[id] = struct{}{}

		blockTransactions, err := idx.tonapiClient.GetBlockchainBlockTransactions(context.Background(), tonapi.GetBlockchainBlockTransactionsParams{id.BlockID.String()})
		if err != nil {
			return nil, err
		}
		ch.blocks = append(ch.blocks, IDandBlock{ID: id, Transactions: blockTransactions.Transactions, BlockTimestamp: block.GenUtime})
	}
	return ch, nil
}
