package g

import (
	"encoding/base64"
	"encoding/hex"
)

func MustHex2Base64(hexHash string) string {
	b, err := hex.DecodeString(hexHash)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}
