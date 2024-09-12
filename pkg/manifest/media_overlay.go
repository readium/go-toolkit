package manifest

// EPUB profile extension for WebPub Manifest for media overlay features.
type MediaOverlay struct {
	ActiveClass         string `json:"activeClass,omitempty"`         // Author-defined CSS class name to apply to the currently-playing EPUB Content Document element.
	PlaybackActiveClass string `json:"playbackActiveClass,omitempty"` // Author-defined CSS class name to apply to the EPUB Content Document's document element when playback is active.
}
