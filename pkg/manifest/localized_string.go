package manifest

import (
	"encoding/json"
	"errors"
)

const UndefinedLanguage = "und"

type LocalizedString struct {
	Translations map[string]string
}

func NewLocalizedStringFromString(value string) LocalizedString {
	ls := LocalizedString{
		Translations: make(map[string]string),
	}
	ls.Translations[UndefinedLanguage] = value
	return ls
}

// Shortcut to create a [LocalizedString] using a map of translations indexed by the BCP 47 language tag.
func NewLocalizedStringFromStrings(strings map[string]string) LocalizedString {
	ls := LocalizedString{
		Translations: make(map[string]string),
	}
	for k, v := range strings {
		if k == "" {
			k = UndefinedLanguage
		}
		ls.Translations[k] = v
	}
	return ls
}

func (l *LocalizedString) String() string {
	return l.DefaultTranslation()
}

func (l *LocalizedString) Length() int {
	return len(l.Translations)
}

func (l *LocalizedString) DefaultTranslation() string {
	return l.GetOrFallback("")
}

func (l *LocalizedString) SetDefaultTranslation(value string) {
	if l.Translations == nil {
		l.Translations = make(map[string]string)
	}
	l.Translations[UndefinedLanguage] = value
}

func (l *LocalizedString) SetTranslation(language string, value string) {
	if l.Translations == nil {
		l.Translations = make(map[string]string)
	}
	if language == "" {
		language = UndefinedLanguage
	}
	l.Translations[language] = value
}

// GetOrFallback Returns the first translation for the given [language] BCP–47 tag.
func (l *LocalizedString) GetOrFallback(language string) string {
	if l == nil {
		return ""
	}

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
	t, ok = l.Translations[UndefinedLanguage]
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
	if len(l.Translations) > 1 {
		return json.Marshal(l.Translations)
	}
	return json.Marshal(l.DefaultTranslation())
}

func LocalizedStringFromJSON(d interface{}) (*LocalizedString, error) {
	if d == nil {
		return nil, errors.New("LocalizedString is nil")
	}
	l := new(LocalizedString)
	switch lsx := d.(type) {
	case string:
		l.SetDefaultTranslation(lsx)
	case map[string]interface{}:
		for k, v := range lsx {
			val, ok := v.(string)
			if !ok {
				return nil, errors.New("LocalizedString value at index " + k + " is not a string")
			}
			l.SetTranslation(k, val)
		}
	default:
		return nil, errors.New("LocalizedString has invalid JSON object")
	}
	return l, nil
}

func (l *LocalizedString) UnmarshalJSON(data []byte) error {
	var d interface{}
	err := json.Unmarshal(data, &d)
	if err != nil {
		return err
	}
	lr, err := LocalizedStringFromJSON(d)
	if err != nil {
		return err
	}
	*l = *lr
	return nil
}
