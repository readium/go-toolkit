package pub

import "encoding/json"

const UNDEFINED_LANGUAGE = "und"

type LocalizedString struct {
	Translations map[string]string
}

func NewLocalizedStringFromString(value string) LocalizedString {
	ls := LocalizedString{
		Translations: make(map[string]string),
	}
	ls.Translations[UNDEFINED_LANGUAGE] = value
	return ls
}

func (l LocalizedString) String() string {
	return l.DefaultTranslation()
}

func (l LocalizedString) DefaultTranslation() string {
	return l.GetOrFallback("")
}

func (l LocalizedString) SetDefaultTranslation(value string) {
	if l.Translations == nil {
		l.Translations = make(map[string]string)
	}
	l.Translations[UNDEFINED_LANGUAGE] = value
}

// GetOrFallback Returns the first translation for the given [language] BCP–47 tag.
func (l LocalizedString) GetOrFallback(language string) string {
	if l.Translations == nil {
		return ""
	}

	t, ok := l.Translations[language]
	if ok {
		return t // Value for the language
	}
	// TODO app locale with viper setting?
	/*t, ok = l.Translations[locale]
	if ok {
		return t
	}*/
	t, ok = l.Translations[UNDEFINED_LANGUAGE]
	if ok {
		return t // "und" value
	}
	t, ok = l.Translations["en"]
	if ok {
		return t // English value
	}
	for _, value := range l.Translations {
		return value // Return first value
	}
	return "" // Should only get here if the map of languages is empty
}

func (l LocalizedString) MarshalJSON() ([]byte, error) {
	if len(l.Translations) > 0 {
		return json.Marshal(l.Translations)
	}
	return json.Marshal(l.DefaultTranslation())
}
