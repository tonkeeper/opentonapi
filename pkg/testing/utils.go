package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func CompareResults(t *testing.T, results any, filenamePrefix string) {
	bs, err := json.MarshalIndent(results, " ", "  ")
	require.Nil(t, err)
	outputName := fmt.Sprintf("testdata/%v.output.json", filenamePrefix)
	err = os.WriteFile(outputName, bs, 0644)
	require.Nil(t, err)
	expected, err := os.ReadFile(fmt.Sprintf("testdata/%v.json", filenamePrefix))
	require.Nil(t, err)
	require.True(t, bytes.Equal(bs, expected))
}
