package client

//
//import (
//	"context"
//	"encoding/base64"
//	"github.com/tonkeeper/tongo"
//	"github.com/tonkeeper/tongo/tlb"
//)
//
//func (c *Client) GetSeqno(ctx context.Context, account tongo.AccountID) (uint32, error) {
//	res, err := c.GetAccountSeqno(ctx, GetAccountSeqnoParams{AccountID: account.ToRaw()})
//	if err != nil {
//		return 0, err
//	}
//	return res.Seqno, nil
//}
//
//func (c *Client) SendMessage(ctx context.Context, payload []byte) (uint32, error) {
//	err := c.SendMessageTemp(ctx, &SendMessageTempReq{Boc: base64.StdEncoding.EncodeToString(payload)})
//	if err != nil {
//		return 0, err
//	}
//	return 0, nil
//}
//
//func (c *Client) GetAccountState(ctx context.Context, accountID tongo.AccountID) (tlb.ShardAccount, error) {
//	s, err := c.GetRawAccount(ctx, GetRawAccountParams{AccountID: accountID.ToRaw()})
//	if err != nil {
//		return tlb.ShardAccount{}, err
//	}
//	var a tlb.ShardAccount
//
//	a.LastTransLt = uint64(s.LastTransactionLt)
//	switch s.Status {
//	case "nonexist":
//		a.Account.SumType = "AccountNone"
//	case "uninit":
//		a.Account.SumType = "Account"
//		a.Account.Account.Addr = accountID.ToMsgAddress()
//		a.Account.Account.Storage.Balance.Grams = tlb.Grams(s.Balance)
//		a.Account.Account.Storage.State.SumType = "AccountUninit"
//	case "active":
//		a.Account.SumType = "Account"
//		a.Account.Account.Addr = accountID.ToMsgAddress()
//		a.Account.Account.Storage.Balance.Grams = tlb.Grams(s.Balance)
//		a.Account.Account.Storage.State.SumType = "AccountActive"
//		a.Account.Account.Storage.State.AccountActive.StateInit.Code.Exists = len(s.Code.Value) > 0
//		a.Account.Account.Storage.State.AccountActive.StateInit.Data.Exists = len(s.Data.Value) > 0
//
//	case "frozen":
//
//	}
//	return a, nil
//
//}
