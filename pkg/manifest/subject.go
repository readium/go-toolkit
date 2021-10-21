package manifest

// Subject
// https://github.com/readium/webpub-manifest/tree/master/contexts/default#subjects
// https://github.com/readium/webpub-manifest/blob/master/schema/subject-object.schema.json
type Subject struct {
	LocalizedName   LocalizedString  `json:"name" validate:"required"`
	LocalizedSortAs *LocalizedString `json:"sortAs,omitempty"`
	Scheme          string           `json:"scheme,omitempty"`
	Code            string           `json:"code,omitempty"`
	Links           []Link           `json:"links,omitempty"`
}

func (s Subject) Name() string {
	return s.LocalizedName.String()
}

func (s Subject) SortAs() string {
	if s.LocalizedSortAs == nil {
		return ""
	}
	return s.LocalizedSortAs.String()
}
