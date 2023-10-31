package opentonapi

//go:generate go run github.com/ogen-go/ogen/cmd/ogen -clean -no-client -package oas -target pkg/oas api/openapi.yml
//go:generate go run github.com/ogen-go/ogen/cmd/ogen -convenient-errors -clean -no-server -package tonapi -target tonapi api/openapi.yml
//go:generate go run api/jsonify.go api/openapi.yml api/openapi.json
