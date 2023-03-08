.PHONY: all fmt test gen

all: gen fmt test

fmt:
	gofmt -s -l -w $$(go list -f {{.Dir}} ./... | grep -v /vendor/)
test:
	go test $$(go list ./... | grep -v /vendor/) -race -coverprofile cover.out
gen:
	go generate
collect_i18n:
	goi18n extract -outdir pkg/i18n/translations -packagepath "github.com/tonkeeper/opentonapi/pkg/i18n" -messagetype M
	#go run github.com/nicksnyder/go-i18n/v2/goi18n extract -outdir pkg/i18n/translations
