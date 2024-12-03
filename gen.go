package opentonapi

//go:generate go run github.com/ogen-go/ogen/cmd/ogen -clean -config ogen.yaml -package oas -target pkg/oas api/openapi.yml
//go:generate go run api/jsonify.go api/openapi.yml api/openapi.json
//go:generate bash -c "cp api/openapi.yml pkg/api/openapi/openapi.yml"
//go:generate bash -c "cp api/openapi.json pkg/api/openapi/openapi.json"
