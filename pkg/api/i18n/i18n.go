package i18n

import (
	"embed"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed translations/active.*.toml
var localeFS embed.FS
var bundle *i18n.Bundle

func init() {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	if _, err := bundle.LoadMessageFileFS(localeFS, "translations/active.en.toml"); err != nil {
		panic(err)
	}
	if _, err := bundle.LoadMessageFileFS(localeFS, "translations/active.ru.toml"); err != nil {
		panic(err)
	}
}

type C = i18n.LocalizeConfig
type M = i18n.Message
type Template = map[string]interface{}

func T(lang string, c C) string {
	s, _ := i18n.NewLocalizer(bundle, lang).Localize(&c)
	// todo: log an error
	return s
}
