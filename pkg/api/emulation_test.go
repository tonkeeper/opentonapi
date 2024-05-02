package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/opentonapi/pkg/addressbook"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

func TestHandler_isEmulationAllowed(t *testing.T) {

	cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
	require.Nil(t, err)

	tests := []struct {
		name        string
		accountID   tongo.AccountID
		addressbook *mockAddressBook
		msgBoc      string
		wantAllowed bool
		wantErr     string
	}{
		{
			name:      "wallet v4r2 - must be allowed",
			accountID: ton.MustParseAccountID("0:6ccd325a858c379693fae2bcaab1c2906831a4e10a6c3bb44ee8b615bca1d220"),
			msgBoc:    "te6ccgEBAwEAqgABRYgA2ZpktQsYby0n9cV5VWOFINBjScIU2HdondFsK3lDpEAMAQGc5gxiLHoFvNnu8mGHOa3gPi2ZOLnLk43DudgThcZ0fv4mU6Wf8BFi1dyZrTtg4m3vnFWBLHAP7Knf4omMkD53DimpoxdmC4IUAAABRgADAgBiQgA2ZpktQsYby0n9cV5VWOFINBjScIU2HdondFsK3lDpEAgIAAAAAAAAAAAAAAAAAA==",
			addressbook: &mockAddressBook{
				OnGetAddressInfoByAddress: func(a tongo.AccountID) (addressbook.KnownAddress, bool) {
					return addressbook.KnownAddress{}, false
				},
			},
			wantAllowed: true,
		},
		{
			name:      "highload wallet - not allowed",
			accountID: ton.MustParseAccountID("0:4d8137641f40829518efb4766fea041de0df89ead2fb785d19de7298c8727a79"),
			msgBoc:    "te6ccgECDwEAAc4AAkWIAbpBee6koPtR+e5jHrgLHFTrydMJhZ8gG54535p0WzXaHgECAgE0AwQBmaNMWTqGJvykmBBWPix3okDP/fiDmNlCKdytZOoDVD+wY3+0J8qnty5I2huJwbsYUElDUA9JkkUGjrKBkJaEDAIpqaMXZguCvj+aDV7ADQEU/wD0pBP0vPLICwUAWSmpoxcAAAAAAAAAAGpZ858uL7Qx8/6yA2ztOonxbsyuJgtOTh/M533mL0JxQAIBIAYHAgFICAkB7vKDCNcYINMf0z/4I6ofUyC58mPtRNDTH9M/0//0BNFTYIBA9A5voTHyYFFzuvKiB/kBVBCH+RDyowL0BNH4AH+OGCGAEPR4b6FvoSCYAtMH1DAB+wCRMuIBs+ZbgyWhyEA0gED0Q4rmMcgSyx8Tyz/L//QAye1UDAAE0DACASAKCwAXvZznaiaGmvmOuF/8AEG+X5dqJoaY+Y6Z/p/5j6AmipEEAgegc30JjJLb/JXdHxQAOCCAQPSWb6FvoTJREJQwUwO53iCTMzYBkjIw4rMBB6AAAAcOAGJCADZmmS1CxhvLSf1xXlVY4Ug0GNJwhTYd2id0WwreUOkQCAgAAAAAAAAAAAAAAAAA",
			addressbook: &mockAddressBook{
				OnGetAddressInfoByAddress: func(a tongo.AccountID) (addressbook.KnownAddress, bool) {
					return addressbook.KnownAddress{}, false
				},
			},
			wantAllowed: false,
		},
		{
			name:      "highload wallet and well known address - allowed",
			accountID: ton.MustParseAccountID("0:4d8137641f40829518efb4766fea041de0df89ead2fb785d19de7298c8727a79"),
			msgBoc:    "te6ccgECDwEAAc4AAkWIAbpBee6koPtR+e5jHrgLHFTrydMJhZ8gG54535p0WzXaHgECAgE0AwQBmaNMWTqGJvykmBBWPix3okDP/fiDmNlCKdytZOoDVD+wY3+0J8qnty5I2huJwbsYUElDUA9JkkUGjrKBkJaEDAIpqaMXZguCvj+aDV7ADQEU/wD0pBP0vPLICwUAWSmpoxcAAAAAAAAAAGpZ858uL7Qx8/6yA2ztOonxbsyuJgtOTh/M533mL0JxQAIBIAYHAgFICAkB7vKDCNcYINMf0z/4I6ofUyC58mPtRNDTH9M/0//0BNFTYIBA9A5voTHyYFFzuvKiB/kBVBCH+RDyowL0BNH4AH+OGCGAEPR4b6FvoSCYAtMH1DAB+wCRMuIBs+ZbgyWhyEA0gED0Q4rmMcgSyx8Tyz/L//QAye1UDAAE0DACASAKCwAXvZznaiaGmvmOuF/8AEG+X5dqJoaY+Y6Z/p/5j6AmipEEAgegc30JjJLb/JXdHxQAOCCAQPSWb6FvoTJREJQwUwO53iCTMzYBkjIw4rMBB6AAAAcOAGJCADZmmS1CxhvLSf1xXlVY4Ug0GNJwhTYd2id0WwreUOkQCAgAAAAAAAAAAAAAAAAA",
			addressbook: &mockAddressBook{
				OnGetAddressInfoByAddress: func(a tongo.AccountID) (addressbook.KnownAddress, bool) {
					return addressbook.KnownAddress{}, true
				},
			},
			wantAllowed: true,
		},
		{
			name:      "highload wallet without init code - not allowed",
			accountID: ton.MustParseAccountID("0:4d8137641f40829518efb4766fea041de0df89ead2fb785d19de7298c8727a79"),
			msgBoc:    "te6ccgEBBAEAsAABRYgBukF57qSg+1H57mMeuAscVOvJ0wmFnyAbnjnfmnRbNdoMAQGZqQhgzHqVoTIbnN+b4dCcES9SeyKzVRfGNrv+5jo0hMoodkZ6wlY53eMi2YMjpNIUmp7u6BeivgHf2LhOjIPAAympoxdmC4LtmOckDsACAQegAAAHAwBiQgA2ZpktQsYby0n9cV5VWOFINBjScIU2HdondFsK3lDpEAgIAAAAAAAAAAAAAAAAAA==",
			addressbook: &mockAddressBook{
				OnGetAddressInfoByAddress: func(a tongo.AccountID) (addressbook.KnownAddress, bool) {
					return addressbook.KnownAddress{}, false
				},
			},
			wantErr: "account is not initialized",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				addressBook: tt.addressbook,
			}
			state, err := cli.GetAccountState(context.Background(), tt.accountID)
			require.Nil(t, err)

			cell, err := boc.DeserializeSinglRootBase64(tt.msgBoc)
			require.Nil(t, err)

			var m tlb.Message
			err = tlb.Unmarshal(cell, &m)
			require.Nil(t, err)

			allowed, err := h.isEmulationAllowed(tt.accountID, state, m)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantAllowed, allowed)
		})
	}
}
