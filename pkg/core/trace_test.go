package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tonkeeper/tongo"
)

func TestTraceAdditionalInfo_JSONKeepsJettonWallets(t *testing.T) {
	wallet := tongo.MustParseAccountID("0:fc35e51a7890d259858bd1caab05052a942006d38e95bee1d8b5045174b63653")
	master := tongo.MustParseAccountID("0:5a01bc2497790051d28ccde06c09b2a43e205d8d089370bf0490bbba093cc5af")
	owner := tongo.MustParseAccountID("0:3996a30dd8d4728d84d8596730bff7d47e2c8ef88de888bf2f6c5aef8df134dc")
	info := &TraceAdditionalInfo{}
	info.SetJettonMaster(wallet, master)
	info.SetJettonOwner(wallet, owner)

	data, err := json.Marshal(info)
	require.NoError(t, err)
	var decoded TraceAdditionalInfo
	require.NoError(t, json.Unmarshal(data, &decoded))

	gotMaster, ok := decoded.JettonMaster(wallet)
	require.True(t, ok)
	require.Equal(t, master, gotMaster)
	gotOwner, ok := decoded.JettonOwner(wallet)
	require.True(t, ok)
	require.Equal(t, owner, gotOwner)

	// traces cached before owners were collected have no JettonOwners
	var old TraceAdditionalInfo
	require.NoError(t, json.Unmarshal([]byte(`{"JettonMasters":{"`+wallet.String()+`":"`+master.String()+`"}}`), &old))
	_, ok = old.JettonOwner(wallet)
	require.False(t, ok)
}
