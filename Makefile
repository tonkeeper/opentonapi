.PHONY: all fmt test gen run

all: gen fmt test

fmt:
	gofmt -s -l -w $$(go list -f {{.Dir}} ./... | grep -v /vendor/)
test:
	which go
	go test $$(go list ./... | grep -v /vendor/) -race -coverprofile cover.out
gen:
	go generate
collect_i18n:
	goi18n extract -outdir pkg/api/i18n/translations -packagepath "github.com/tonkeeper/opentonapi/pkg/api/i18n" -messagetype M
	#go run github.com/nicksnyder/go-i18n/v2/goi18n extract -outdir pkg/api/i18n/translations todo: switch to this version after https://github.com/nicksnyder/go-i18n/pull/295
translate:
	goi18n merge -outdir pkg/api/i18n/translations/ pkg/api/i18n/translations/active.en.toml  pkg/api/i18n/translations/active.ru.toml

install_i18n:
	git clone https://github.com/mr-tron/go-i18n/
	cd go-i18n/v2 && go build -o $GOPATH/bin/goi18n github.com/nicksnyder/go-i18n/v2/goi18n
run:
	go run cmd/api/main.go


TMPDIR := $(shell mktemp -d)
update-sdk:
	git clone git@github.com:tonkeeper/tonapi-go.git $(TMPDIR) \
		&& cp -v api/openapi.yml $(TMPDIR)/api/openapi.yml \
		&& cd $(TMPDIR) \
		&& git checkout -b update \
		&& go generate \
		&& git add . \
		&& git commit -m "update sdk" \
		&& git push -u origin update -f \
		&& gh pr create --title "Update TonAPI SDK" --body "This PR was created automatically" --head update --base main

