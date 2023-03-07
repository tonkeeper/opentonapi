.PHONY: all fmt test gen

all: gen fmt test

fmt:
	gofmt -s -l -w $$(go list -f {{.Dir}} ./... | grep -v /vendor/)
test:
	go test $$(go list ./... | grep -v /vendor/) -race -coverprofile cover.out
gen:
	go generate

