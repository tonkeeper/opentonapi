package defi

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/*
var defiAssetsFS embed.FS

func AssetsHandler() http.Handler {
	assets, err := fs.Sub(defiAssetsFS, "assets")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/v2/assets/defi/", http.FileServer(http.FS(assets)))
}
