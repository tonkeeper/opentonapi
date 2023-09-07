package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tonkeeper/opentonapi/pkg/litestorage"
	"github.com/tonkeeper/opentonapi/pkg/oas"
)

func TestHandler_EmulateMessageToAccountEvent(t *testing.T) {
	tests := []struct {
		name        string
		request     oas.EmulateMessageToAccountEventReq
		params      oas.EmulateMessageToAccountEventParams
		wantActions []oas.ActionType
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			liteStorage, err := litestorage.NewLiteStorage(logger)
			require.Nil(t, err)
			h, err := NewHandler(logger, WithStorage(liteStorage), WithExecutor(liteStorage))
			require.Nil(t, err)

			got, err := h.EmulateMessageToAccountEvent(context.Background(), &tt.request, tt.params)
			require.Nil(t, err)

			var actions []oas.ActionType
			for _, action := range got.Actions {
				actions = append(actions, action.Type)
			}
			require.Equal(t, tt.wantActions, actions)
		})
	}
}
