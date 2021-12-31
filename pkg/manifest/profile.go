package manifest

import "encoding/json"

/*
 * Represents a Readium Web Publication Profile a [Publication] can conform to.
 * For a list of supported profiles, see the registry:
 * https://readium.org/webpub-manifest/profiles/
 */

type Profile string

// Profile for EPUB publications
const ProfileEPUB = Profile("https://readium.org/webpub-manifest/profiles/epub")

// Profile for audiobooks
const ProfileAudiobook = Profile("https://readium.org/webpub-manifest/profiles/audiobook")

// Profile for visual narratives (comics, manga and bandes dessinées)
const ProfileDivina = Profile("https://readium.org/webpub-manifest/profiles/divina")

// Profile for PDF documents
const ProfilePDF = Profile("https://readium.org/webpub-manifest/profiles/pdf")

// TODO replace with generic
type Profiles []Profile

// Convert to string slice
func (p Profiles) toss() []string {
	return interface{}(p).([]string) // Ugly...
}

func (p Profiles) MarshalJSON() ([]byte, error) {
	if len(p) == 1 {
		return json.Marshal(p[0])
	}
	type alias Profiles
	return json.Marshal(alias(p))
}

// Ugly workaround
func profilesFromStrings(raw []string) []Profile {
	profiles := make([]Profile, len(raw))
	for i, p := range raw {
		profiles[i] = Profile(p)
	}
	return profiles
}
