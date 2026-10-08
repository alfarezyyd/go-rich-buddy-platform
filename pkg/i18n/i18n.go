package i18n

import (
	"embed"
	"encoding/json"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

// Bundle is the global i18n bundle holding all loaded locale messages.
type Bundle struct {
	bundle *i18n.Bundle
}

// NewBundle loads all locale JSON files from the embedded /locales/ directory
// and returns an initialized Bundle. The default language is Indonesian (id).
func NewBundle() *Bundle {
	bundle := i18n.NewBundle(language.Indonesian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	files := []string{
		"locales/id.json",
		"locales/en.json",
	}
	for _, f := range files {
		if _, err := bundle.LoadMessageFileFS(localeFS, f); err != nil {
			panic("i18n: failed to load locale file " + f + ": " + err.Error())
		}
	}

	return &Bundle{bundle: bundle}
}

// Localizer wraps go-i18n's Localizer and exposes a simplified API for
// translating messages with optional template data.
type Localizer struct {
	inner *i18n.Localizer
}

// NewLocalizer creates a Localizer for the given language tag (e.g. "id", "en").
// Falls back to the bundle's default language when the tag is not found.
func (b *Bundle) NewLocalizer(lang string) *Localizer {
	return &Localizer{
		inner: i18n.NewLocalizer(b.bundle, lang),
	}
}

// T translates a message by its ID with no template data.
func (l *Localizer) T(messageID string) string {
	msg, err := l.inner.Localize(&i18n.LocalizeConfig{MessageID: messageID})
	if err != nil {
		return messageID // graceful degradation: return the key itself
	}
	return msg
}

// TData translates a message by its ID and interpolates the provided template data.
// templateData should be a map or struct whose fields match the {{.Field}} placeholders
// in the locale JSON.
func (l *Localizer) TData(messageID string, templateData any) string {
	msg, err := l.inner.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	})
	if err != nil {
		return messageID
	}
	return msg
}

// TPlural translates a plural-aware message (e.g. "watchlist_add_success").
// pluralCount is used to select "one" vs "other" form, and templateData
// provides the interpolation values.
func (l *Localizer) TPlural(messageID string, pluralCount int, templateData any) string {
	msg, err := l.inner.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		PluralCount:  pluralCount,
		TemplateData: templateData,
	})
	if err != nil {
		return messageID
	}
	return msg
}
