// Code generated by ogen, DO NOT EDIT.

package oas

import (
	"context"

	ht "github.com/ogen-go/ogen/http"
)

// UnimplementedHandler is no-op Handler which returns http.ErrNotImplemented.
type UnimplementedHandler struct{}

var _ Handler = UnimplementedHandler{}

// DnsBackResolve implements dnsBackResolve operation.
//
// Get domains for wallet account.
//
// GET /v2/accounts/{account_id}/dns/backresolve
func (UnimplementedHandler) DnsBackResolve(ctx context.Context, params DnsBackResolveParams) (r DnsBackResolveRes, _ error) {
	return r, ht.ErrNotImplemented
}

// DnsResolve implements dnsResolve operation.
//
// DNS resolve for domain name.
//
// GET /v2/dns/{domain_name}/resolve
func (UnimplementedHandler) DnsResolve(ctx context.Context, params DnsResolveParams) (r DnsResolveRes, _ error) {
	return r, ht.ErrNotImplemented
}

// EmulateMessage implements emulateMessage operation.
//
// Emulate sending message to blockchain.
//
// POST /v2/blockchain/message/emulate
func (UnimplementedHandler) EmulateMessage(ctx context.Context, req OptEmulateMessageReq) (r EmulateMessageRes, _ error) {
	return r, ht.ErrNotImplemented
}

// ExecGetMethod implements execGetMethod operation.
//
// Execute get method for account.
//
// GET /v2/blockchain/accounts/{account_id}/methods/{method_name}
func (UnimplementedHandler) ExecGetMethod(ctx context.Context, params ExecGetMethodParams) (r ExecGetMethodRes, _ error) {
	return r, ht.ErrNotImplemented
}

// ExecGetMethodPost implements execGetMethodPost operation.
//
// Execute get method for account.
//
// POST /v2/blockchain/accounts/{account_id}/methods/{method_name}
func (UnimplementedHandler) ExecGetMethodPost(ctx context.Context, req *ExecGetMethodPostReq, params ExecGetMethodPostParams) (r ExecGetMethodPostRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetAccount implements getAccount operation.
//
// Get human-friendly information about an account without low-level details.
//
// GET /v2/accounts/{account_id}
func (UnimplementedHandler) GetAccount(ctx context.Context, params GetAccountParams) (r GetAccountRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetAccountTransactions implements getAccountTransactions operation.
//
// Get account transactions.
//
// GET /v2/blockchain/accounts/{account_id}/transactions
func (UnimplementedHandler) GetAccountTransactions(ctx context.Context, params GetAccountTransactionsParams) (r GetAccountTransactionsRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetAllAuctions implements getAllAuctions operation.
//
// Get all auctions.
//
// GET /v2/dns/auctions
func (UnimplementedHandler) GetAllAuctions(ctx context.Context, params GetAllAuctionsParams) (r GetAllAuctionsRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetBlock implements getBlock operation.
//
// Get block data.
//
// GET /v2/blockchain/blocks/{block_id}
func (UnimplementedHandler) GetBlock(ctx context.Context, params GetBlockParams) (r GetBlockRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetBlockTransactions implements getBlockTransactions operation.
//
// Get transactions from block.
//
// GET /v2/blockchain/blocks/{block_id}/transactions
func (UnimplementedHandler) GetBlockTransactions(ctx context.Context, params GetBlockTransactionsParams) (r GetBlockTransactionsRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetConfig implements getConfig operation.
//
// Get blockchain config.
//
// GET /v2/blockchain/config
func (UnimplementedHandler) GetConfig(ctx context.Context) (r GetConfigRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetDomainBids implements getDomainBids operation.
//
// Get domain bids.
//
// GET /v2/dns/{domain_name}/bids
func (UnimplementedHandler) GetDomainBids(ctx context.Context, params GetDomainBidsParams) (r GetDomainBidsRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetEvent implements getEvent operation.
//
// Get the event by event ID or hash of any transaction in trace.
//
// GET /v2/events/{event_id}
func (UnimplementedHandler) GetEvent(ctx context.Context, params GetEventParams) (r GetEventRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetEventsByAccount implements getEventsByAccount operation.
//
// Get events for account.
//
// GET /v2/accounts/{account_id}/events
func (UnimplementedHandler) GetEventsByAccount(ctx context.Context, params GetEventsByAccountParams) (r GetEventsByAccountRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetJettonInfo implements getJettonInfo operation.
//
// Get jetton metadata by jetton master address.
//
// GET /v2/jettons/{account_id}
func (UnimplementedHandler) GetJettonInfo(ctx context.Context, params GetJettonInfoParams) (r GetJettonInfoRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetJettonsBalances implements getJettonsBalances operation.
//
// Get all Jettons balances by owner address.
//
// GET /v2/accounts/{account_id}/jettons
func (UnimplementedHandler) GetJettonsBalances(ctx context.Context, params GetJettonsBalancesParams) (r GetJettonsBalancesRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetMasterchainHead implements getMasterchainHead operation.
//
// Get last known masterchain block.
//
// GET /v2/blockchain/masterchain-head
func (UnimplementedHandler) GetMasterchainHead(ctx context.Context) (r GetMasterchainHeadRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetNftCollection implements getNftCollection operation.
//
// Get NFT collection by collection address.
//
// GET /v2/nfts/collections/{account_id}
func (UnimplementedHandler) GetNftCollection(ctx context.Context, params GetNftCollectionParams) (r GetNftCollectionRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetNftCollections implements getNftCollections operation.
//
// Get NFT collections.
//
// GET /v2/nfts/collections
func (UnimplementedHandler) GetNftCollections(ctx context.Context, params GetNftCollectionsParams) (r GetNftCollectionsRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetNftItemsByAddresses implements getNftItemsByAddresses operation.
//
// Get NFT items by its address.
//
// GET /v2/nfts/{account_ids}
func (UnimplementedHandler) GetNftItemsByAddresses(ctx context.Context, params GetNftItemsByAddressesParams) (r GetNftItemsByAddressesRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetNftItemsByOwner implements getNftItemsByOwner operation.
//
// Get all NFT items by owner address.
//
// GET /v2/accounts/{account_id}/ntfs
func (UnimplementedHandler) GetNftItemsByOwner(ctx context.Context, params GetNftItemsByOwnerParams) (r GetNftItemsByOwnerRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetRawAccount implements getRawAccount operation.
//
// Get low-level information about an account taken directly from the blockchain.
//
// GET /v2/blockchain/accounts/{account_id}
func (UnimplementedHandler) GetRawAccount(ctx context.Context, params GetRawAccountParams) (r GetRawAccountRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetSubscriptionsByAccount implements getSubscriptionsByAccount operation.
//
// Get all subscriptions by wallet address.
//
// GET /v2/accounts/{account_id}/subscriptions
func (UnimplementedHandler) GetSubscriptionsByAccount(ctx context.Context, params GetSubscriptionsByAccountParams) (r GetSubscriptionsByAccountRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetTrace implements getTrace operation.
//
// Get the trace by trace ID or hash of any transaction in trace.
//
// GET /v2/traces/{trace_id}
func (UnimplementedHandler) GetTrace(ctx context.Context, params GetTraceParams) (r GetTraceRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetTracesByAccount implements getTracesByAccount operation.
//
// Get traces for account.
//
// GET /v2/accounts/{account_id}/traces
func (UnimplementedHandler) GetTracesByAccount(ctx context.Context, params GetTracesByAccountParams) (r GetTracesByAccountRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetTransaction implements getTransaction operation.
//
// Get transaction data.
//
// GET /v2/blockchain/transactions/{transaction_id}
func (UnimplementedHandler) GetTransaction(ctx context.Context, params GetTransactionParams) (r GetTransactionRes, _ error) {
	return r, ht.ErrNotImplemented
}

// GetValidators implements getValidators operation.
//
// Get validators.
//
// GET /v2/blockchain/validators
func (UnimplementedHandler) GetValidators(ctx context.Context) (r GetValidatorsRes, _ error) {
	return r, ht.ErrNotImplemented
}

// PoolsByNominators implements poolsByNominators operation.
//
// All pools where account participates.
//
// GET /v2/staking/nominator/{account_id}/pools
func (UnimplementedHandler) PoolsByNominators(ctx context.Context, params PoolsByNominatorsParams) (r PoolsByNominatorsRes, _ error) {
	return r, ht.ErrNotImplemented
}

// SendMessage implements sendMessage operation.
//
// Send message to blockchain.
//
// POST /v2/blockchain/message
func (UnimplementedHandler) SendMessage(ctx context.Context, req OptSendMessageReq) (r SendMessageRes, _ error) {
	return r, ht.ErrNotImplemented
}

// StakingPoolInfo implements stakingPoolInfo operation.
//
// Pool info.
//
// GET /v2/staking/pool/{account_id}
func (UnimplementedHandler) StakingPoolInfo(ctx context.Context, params StakingPoolInfoParams) (r StakingPoolInfoRes, _ error) {
	return r, ht.ErrNotImplemented
}

// StakingPools implements stakingPools operation.
//
// All pools available in network.
//
// GET /v2/staking/pools
func (UnimplementedHandler) StakingPools(ctx context.Context, params StakingPoolsParams) (r StakingPoolsRes, _ error) {
	return r, ht.ErrNotImplemented
}
