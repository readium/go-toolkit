package pub

import (
	"encoding/json"
	"errors"
)

const UNDEFINED_LANGUAGE = "und"

type LocalizedString struct {
	translations map[string]string
}

func NewLocalizedStringFromString(value string) LocalizedString {
	ls := LocalizedString{
		translations: make(map[string]string),
	}
	ls.translations[UNDEFINED_LANGUAGE] = value
	return ls
}

func (l *LocalizedString) String() string {
	return l.DefaultTranslation()
}

func (l *LocalizedString) DefaultTranslation() string {
	return l.GetOrFallback("")
}

func (l *LocalizedString) SetDefaultTranslation(value string) {
	if l.translations == nil {
		l.translations = make(map[string]string)
	}
	l.translations[UNDEFINED_LANGUAGE] = value
}

func (l *LocalizedString) SetTranslation(language string, value string) {
	if l.translations == nil {
		l.translations = make(map[string]string)
	}
	l.translations[language] = value
}

// GetOrFallback Returns the first translation for the given [language] BCP–47 tag.
func (l *LocalizedString) GetOrFallback(language string) string {
	if l.translations == nil {
		return ""
	}

	t, ok := l.translations[language]
	if ok {
		return t // Value for the language
	}
	// TODO app locale with viper setting?
	/*t, ok = l.Translations[locale]
	if ok {
		return t
	}*/
	t, ok = l.translations[UNDEFINED_LANGUAGE]
	if ok {
		return t // "und" value
	}
	t, ok = l.translations["en"]
	if ok {
		return t // English value
	}
	for _, value := range l.translations {
		return value // Return first value
	}
	return "" // Should only get here if the map of languages is empty
}

func (l LocalizedString) MarshalJSON() ([]byte, error) {
	if len(l.translations) > 1 {
		return json.Marshal(l.translations)
	}
	return json.Marshal(l.DefaultTranslation())
}

func (l *LocalizedString) UnmarshalJSON(data []byte) error {
	var d interface{}
	err := json.Unmarshal(data, &d)
	if err != nil {
		return err
	}
	switch d.(type) {
	case string:
		l.SetDefaultTranslation(d.(string))
	case map[string]interface{}:
		for k, v := range d.(map[string]interface{}) {
			val, ok := v.(string)
			if !ok {
				return errors.New("LocalizedString value for " + k + " is not a string")
			}
			l.SetTranslation(k, val)
		}
	default:
		return errors.New("LocalizedString has invalid JSON object")
	}
	return nil
}
