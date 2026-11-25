package protection

import (
	"context"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/pub"
)

// ContentProtection unlocks the publications protected with a DRM scheme, so
// that the streamer can open them.
type ContentProtection interface {
	// Open is called by the streamer before parsing an asset, with the
	// asset's fetcher and the credentials given by the caller, e.g. a
	// passphrase.
	//
	// It returns nil, without error, when the asset is not protected with
	// the scheme it handles. It returns an error when the asset is protected
	// with that scheme but can't be unlocked, e.g. because the credentials
	// are wrong.
	Open(ctx context.Context, a asset.PublicationAsset, f fetcher.Fetcher, credentials string) (*ProtectedAsset, error)
}

// ProtectedAsset is an asset unlocked by a [ContentProtection].
type ProtectedAsset struct {
	// Scheme is the protection scheme of the asset.
	Scheme Scheme

	// Fetcher, when not nil, replaces the asset's fetcher for parsing the
	// publication. It is responsible for closing the original fetcher.
	Fetcher fetcher.Fetcher

	// OnCreatePublication, when not nil, is called with the builder of the
	// parsed publication before it is built, e.g. to decrypt its resources.
	// It is called before the streamer's own OnCreatePublication.
	OnCreatePublication func(*pub.Builder) error
}
