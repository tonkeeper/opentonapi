package client

import (
	"context"
	"encoding/base64"

	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
)

func (c *Client) GetSeqno(ctx context.Context, account tongo.AccountID) (uint32, error) {
	res, err := c.GetAccountSeqno(ctx, GetAccountSeqnoParams{AccountID: account.ToRaw()})
	if err != nil {
		return 0, err
	}
	return res.Seqno, nil
}

func (c *Client) SendMessage(ctx context.Context, payload []byte) (uint32, error) {
	var req SendBlockchainMessageReq
	req.Boc.SetTo(base64.StdEncoding.EncodeToString(payload))
	err := c.SendBlockchainMessage(ctx, &req)
	if err != nil {
		return 0, err
	}
	return 0, nil
}

func (c *Client) GetAccountState(ctx context.Context, accountID tongo.AccountID) (tlb.ShardAccount, error) {
	res, err := c.GetBlockchainRawAccount(ctx, GetBlockchainRawAccountParams{AccountID: accountID.ToRaw()})
	if err != nil {
		return tlb.ShardAccount{}, err
	}
	var shardAccount tlb.ShardAccount
	shardAccount.LastTransLt = uint64(res.LastTransactionLt)
	switch res.Status {
	case "nonexist":
		shardAccount.Account.SumType = "AccountNone"
	case "uninit":
		shardAccount.Account.SumType = "Account"
		shardAccount.Account.Account.Addr = accountID.ToMsgAddress()
		shardAccount.Account.Account.Storage.Balance.Grams = tlb.Grams(res.Balance)
		shardAccount.Account.Account.Storage.State.SumType = "AccountUninit"
	case "active":
		shardAccount.Account.SumType = "Account"
		shardAccount.Account.Account.Addr = accountID.ToMsgAddress()
		shardAccount.Account.Account.Storage.Balance.Grams = tlb.Grams(res.Balance)
		shardAccount.Account.Account.Storage.State.SumType = "AccountActive"
		shardAccount.Account.Account.Storage.State.AccountActive.StateInit.Code.Exists = len(res.Code.Value) > 0
		shardAccount.Account.Account.Storage.State.AccountActive.StateInit.Data.Exists = len(res.Data.Value) > 0
	case "frozen":

	}
	return shardAccount, nil
}
