package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/tonkeeper/opentonapi/pkg/config"
	"log"
)

func GenerateImageUrl(url string, height, width int) string {
	var keyBin, saltBin []byte
	var err error

	if keyBin, err = hex.DecodeString(config.ImageProxy.ImageProxyKey); err != nil {
		log.Fatal("Key expected to be hex-encoded string")
	}

	if saltBin, err = hex.DecodeString(config.ImageProxy.ImageProxySalt); err != nil {
		log.Fatal("Salt expected to be hex-encoded string")
	}

	resize := "fill"
	gravity := "no"
	enlarge := 1
	extension := "webp"

	encodedURL := base64.RawURLEncoding.EncodeToString([]byte(url))

	path := fmt.Sprintf("/rs:%s:%d:%d:%d/g:%s/%s.%s", resize, width, height, enlarge, gravity, encodedURL, extension)

	mac := hmac.New(sha256.New, keyBin)
	mac.Write(saltBin)
	mac.Write([]byte(path))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return config.ImageProxy.CacheWarmupPath + signature + path
}
