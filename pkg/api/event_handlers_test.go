package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"go.uber.org/zap"

	"github.com/arnac-io/opentonapi/pkg/litestorage"
	"github.com/arnac-io/opentonapi/pkg/oas"
)

func TestHandler_EmulateMessageToAccountEvent(t *testing.T) {
	tests := []struct {
		name          string
		request       oas.EmulateMessageToAccountEventReq
		params        oas.EmulateMessageToAccountEventParams
		wantActions   []oas.ActionType
		wantErrorCode int
	}{
		{
			name: "all good",
			request: oas.EmulateMessageToAccountEventReq{
				Boc: "te6ccgECGwEABC4AAkWIAZbd4qXkLyRXrjVN2JQ9x7JfWdNKbVHPasEGngPMWDMSHgECAgE0AwQBnPDbqqyy68IUa0Nm6WG3ETilFHf4QCzKFlQdfA4cM9bXjBujpPoHaKoYbOSBb/lJg8OX6iKMjALeHY/bavOKTwspqaMXd8XhEwAAAAAAARgBFP8A9KQT9LzyyAsFAFEAAAAAKamjF7A9F69p3THnzsezxrW8DD0lnMnKzJ3l6Ccx8S7qAgXAQAIBIAYHAgFICAkE+PKDCNcYINMf0x/THwL4I7vyZO1E0NMf0x/T//QE0VFDuvKhUVG68qIF+QFUEGT5EPKj+AAkpMjLH1JAyx9SMMv/UhD0AMntVPgPAdMHIcAAn2xRkyDXSpbTB9QC+wDoMOAhwAHjACHAAuMAAcADkTDjDQOkyMsfEssfy/8UFRYXAubQAdDTAyFxsJJfBOAi10nBIJJfBOAC0x8hghBwbHVnvSKCEGRzdHK9sJJfBeAD+kAwIPpEAcjKB8v/ydDtRNCBAUDXIfQEMFyBAQj0Cm+hMbOSXwfgBdM/yCWCEHBsdWe6kjgw4w0DghBkc3RyupJfBuMNCgsCASAMDQB4AfoA9AQw+CdvIjBQCqEhvvLgUIIQcGx1Z4MesXCAGFAEywUmzxZY+gIZ9ADLaRfLH1Jgyz8gyYBA+wAGAIpQBIEBCPRZMO1E0IEBQNcgyAHPFvQAye1UAXKwjiOCEGRzdHKDHrFwgBhQBcsFUAPPFiP6AhPLassfyz/JgED7AJJfA+ICASAODwBZvSQrb2omhAgKBrkPoCGEcNQICEekk30pkQzmkD6f+YN4EoAbeBAUiYcVnzGEAgFYEBEAEbjJftRNDXCx+AA9sp37UTQgQFA1yH0BDACyMoHy//J0AGBAQj0Cm+hMYAIBIBITABmtznaiaEAga5Drhf/AABmvHfaiaEAQa5DrhY/AAG7SB/oA1NQi+QAFyMoHFcv/ydB3dIAYyMsFywIizxZQBfoCFMtrEszMyXP7AMhAFIEBCPRR8qcCAHCBAQjXGPoA0z/IVCBHgQEI9FHyp4IQbm90ZXB0gBjIywXLAlAGzxZQBPoCFMtqEssfyz/Jc/sAAgBsgQEI1xj6ANM/MFIkgQEI9Fnyp4IQZHN0cnB0gBjIywXLAlAFzxZQA/oCE8tqyx8Syz/Jc/sAAAr0AMntVAGrCAGW3eKl5C8kV641TdiUPceyX1nTSm1Rz2rBBp4DzFgzEwAdQno8lZqL/+Y1ecbvwTF4/br+jDDDSniikcHI4b3sOVBHhowAAAAAAAAAAAAAAAAAAMAZAagPin6leJnEQ6d06sRDuaygCADZmmS1CxhvLSf1xXlVY4Ug0GNJwhTYd2id0WwreUOkQQAy27xUvIXkivXGqbsSh7j2S+s6aU2qOe1YINPAeYsGYkEaAAA=",
			},
			params: oas.EmulateMessageToAccountEventParams{
				AccountID: "0:cb6ef152f217922bd71aa6ec4a1ee3d92face9a536a8e7b560834f01e62c1989",
			},
			wantActions: []oas.ActionType{
				oas.ActionTypeContractDeploy,
			},
		},
		{
			name: "all good - account requires a public library",
			request: oas.EmulateMessageToAccountEventReq{
				Boc: "te6cckEBAgEAoAABz4gBahvXA1d597WU+VHmK/4MThUfY4WrH36i9HDzIYRSKR4EvarVUSxQjIWI94OkBEM96M0tkECkPU4K/yg+X2XGUZ6o7/SZ7Msx2G0QAL3xVP9IBo2AVy/VXqkOwkdy8C4oUAAAABAcAQBmYgA2ZpktQsYby0n9cV5VWOFINBjScIU2HdondFsK3lDpEBgMNQAAAAAAAAAAAAAAAAAAUlsD/w==",
			},
			params: oas.EmulateMessageToAccountEventParams{
				AccountID: "EQC1DeuBq7z72sp8qPMV_wYnCo-xwtWPv1F6OHmQwikUj-cH",
			},
			wantActions: []oas.ActionType{
				oas.ActionTypeTonTransfer,
			},
		},
		{
			name: "stonfi swap",
			request: oas.EmulateMessageToAccountEventReq{
				Boc: "te6ccgECBQEAAVQAAUWIACH3HsOIrW6qYEGULYwYelIP1FWIbBZWR76TWN6T09tqDAEBnGpXvpSE1bg/EO+niFDpM0t2dr4ixdbn9pAjhkXp39EvtfL99jwb1PftZOBwO8Def181ywRne4BYs5hixmeDcAMpqaMXbo4DyQAAAAMAAwIBqwgAIfcew4itbqpgQZQtjBh6Ug/UVYhsFlZHvpNY3pPT22sAPiDtbeq0MAbjINMSK7AKe4gRyuvoVE1m1xeXoS+XZAsQR4aMAAAAAAAAAAAAAAAAAADAAwFpD4p+pdp2wn0DXAp7ID6IAO87mQKicbKgHIk4pSPP4k5xhHqutqYgAB7USnesDnCcEDk4cAMEAJMlk4VhgBD3JEg1TUr75iTijBghOKm/sxNDXUBl7CD6WMut0Q85xAfRAAQ+49hxFa3VTAgyhbGDD0pB+oqxDYLKyPfSaxvSenttUA==",
			},
			params: oas.EmulateMessageToAccountEventParams{
				AccountID: "0:10fb8f61c456b7553020ca16c60c3d2907ea2ac4360b2b23df49ac6f49e9edb5",
			},
			wantActions: []oas.ActionType{
				oas.ActionTypeJettonSwap,
			},
		},
		{
			name: "message is not accepted by account",
			request: oas.EmulateMessageToAccountEventReq{
				Boc: "te6ccgEBAwEAqwABRYgA2ZpktQsYby0n9cV5VWOFINBjScIU2HdondFsK3lDpEAMAQGcLYVC3cK/Ulyr7Sa1rxw0XGsG56ppG9v7XscYE8iFztO5+MzcdmMOjhSdgpIfA+6SZL5PTZiCof6dw8G4W049BCmpoxdlea4wAAABBwADAgBkQgAWedrcZEjyi+TW7e0OAcNQTmVl2HH9exhajn5AqRktZZAfQAAAAAAAAAAAAAAAAAA=",
			},
			params: oas.EmulateMessageToAccountEventParams{
				AccountID: "0:6ccd325a858c379693fae2bcaab1c2906831a4e10a6c3bb44ee8b615bca1d220",
			},
			wantErrorCode: http.StatusNotAcceptable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
			require.Nil(t, err)
			liteStorage, err := litestorage.NewLiteStorage(logger, cli)
			require.Nil(t, err)
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
			require.Nil(t, err)

			got, err := h.EmulateMessageToAccountEvent(context.Background(), &tt.request, tt.params)
			if tt.wantErrorCode != 0 {
				require.Equal(t, tt.wantErrorCode, err.(*oas.ErrorStatusCode).StatusCode)
				return
			}
			require.Nil(t, err)

			var actions []oas.ActionType
			for _, action := range got.Actions {
				actions = append(actions, action.Type)
			}
			require.Equal(t, tt.wantActions, actions)
		})
	}
}

func Test_prepareAccountState(t *testing.T) {
	cli, err := liteapi.NewClient(liteapi.FromEnvsOrMainnet())
	require.Nil(t, err)

	tests := []struct {
		name         string
		accountID    string
		startBalance int64
		wantStatus   tlb.AccountStatus
	}{
		{
			name:         "uninit account",
			accountID:    "EQBszTJahYw3lpP64ryqscKQaDGk4QpsO7RO6LYVvKHSIvT2",
			startBalance: 100500,
			wantStatus:   tlb.AccountUninit,
		},
		{
			name:         "existing account",
			accountID:    "0:6ccd325a858c379693fae2bcaab1c2906831a4e10a6c3bb44ee8b615bca1d220",
			startBalance: 500_000,
			wantStatus:   tlb.AccountActive,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accountID, err := ton.ParseAccountID(tt.accountID)
			require.Nil(t, err)
			state, err := cli.GetAccountState(context.Background(), accountID)
			require.Nil(t, err)
			account, err := prepareAccountState(accountID, state, tt.startBalance)
			require.Nil(t, err)
			require.Equal(t, tt.wantStatus, account.Account.Status())
			require.Equal(t, tlb.SumType("Account"), account.Account.SumType)
			require.Equal(t, tt.startBalance, int64(account.Account.Account.Storage.Balance.Grams))
		})
	}
}

func mustFromHex(s string) []byte {
	value, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return value
}

func Test_decodeMessage(t *testing.T) {
	tests := []struct {
		name     string
		msgValue string
		want     *decodedMessage
		wantErr  bool
	}{
		{
			name:     "base64",
			msgValue: "te6ccgEBAwEAqwABRYgA2ZpktQsYby0n9cV5VWOFINBjScIU2HdondFsK3lDpEAMAQGcLYVC3cK/Ulyr7Sa1rxw0XGsG56ppG9v7XscYE8iFztO5+MzcdmMOjhSdgpIfA+6SZL5PTZiCof6dw8G4W049BCmpoxdlea4wAAABBwADAgBkQgAWedrcZEjyi+TW7e0OAcNQTmVl2HH9exhajn5AqRktZZAfQAAAAAAAAAAAAAAAAAA=",
			want: &decodedMessage{
				base64:  "te6ccgEBAwEAqwABRYgA2ZpktQsYby0n9cV5VWOFINBjScIU2HdondFsK3lDpEAMAQGcLYVC3cK/Ulyr7Sa1rxw0XGsG56ppG9v7XscYE8iFztO5+MzcdmMOjhSdgpIfA+6SZL5PTZiCof6dw8G4W049BCmpoxdlea4wAAABBwADAgBkQgAWedrcZEjyi+TW7e0OAcNQTmVl2HH9exhajn5AqRktZZAfQAAAAAAAAAAAAAAAAAA=",
				payload: mustFromHex("b5ee9c720101030100ab0001458800d99a64b50b186f2d27f5c57955638520d06349c214d877689dd16c2b7943a4400c01019c2d8542ddc2bf525cabed26b5af1c345c6b06e7aa691bdbfb5ec71813c885ced3b9f8ccdc76630e8e149d82921f03ee9264be4f4d9882a1fe9dc3c1b85b4e3d0429a9a3176579ae3000000107000302006442001679dadc6448f28be4d6eded0e01c3504e6565d871fd7b185a8e7e40a9192d65901f4000000000000000000000000000"),
			},
		},
		{
			name:     "hex",
			msgValue: "b5ee9c720101030100ab0001458800d99a64b50b186f2d27f5c57955638520d06349c214d877689dd16c2b7943a4400c01019c2d8542ddc2bf525cabed26b5af1c345c6b06e7aa691bdbfb5ec71813c885ced3b9f8ccdc76630e8e149d82921f03ee9264be4f4d9882a1fe9dc3c1b85b4e3d0429a9a3176579ae3000000107000302006442001679dadc6448f28be4d6eded0e01c3504e6565d871fd7b185a8e7e40a9192d65901f4000000000000000000000000000",
			want: &decodedMessage{
				base64:  "te6ccgEBAwEAqwABRYgA2ZpktQsYby0n9cV5VWOFINBjScIU2HdondFsK3lDpEAMAQGcLYVC3cK/Ulyr7Sa1rxw0XGsG56ppG9v7XscYE8iFztO5+MzcdmMOjhSdgpIfA+6SZL5PTZiCof6dw8G4W049BCmpoxdlea4wAAABBwADAgBkQgAWedrcZEjyi+TW7e0OAcNQTmVl2HH9exhajn5AqRktZZAfQAAAAAAAAAAAAAAAAAA=",
				payload: mustFromHex("b5ee9c720101030100ab0001458800d99a64b50b186f2d27f5c57955638520d06349c214d877689dd16c2b7943a4400c01019c2d8542ddc2bf525cabed26b5af1c345c6b06e7aa691bdbfb5ec71813c885ced3b9f8ccdc76630e8e149d82921f03ee9264be4f4d9882a1fe9dc3c1b85b4e3d0429a9a3176579ae3000000107000302006442001679dadc6448f28be4d6eded0e01c3504e6565d871fd7b185a8e7e40a9192d65901f4000000000000000000000000000"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeMessage(tt.msgValue)
			fmt.Printf("got: %x\n", got.payload)
			require.Nil(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
