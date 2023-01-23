package fetcher

import "github.com/readium/go-toolkit/pkg/manifest"

// Transforms the resources' content of a child fetcher using a list of [ResourceTransformer] functions.
type TransformingFetcher struct {
	fetcher      Fetcher
	transformers []ResourceTransformer
}

// Links implements Fetcher
func (f *TransformingFetcher) Links() (manifest.LinkList, error) {
	return f.fetcher.Links()
}

// Get implements Fetcher
func (f *TransformingFetcher) Get(link manifest.Link) Resource {
	resource := f.fetcher.Get(link)
	for _, transformer := range f.transformers {
		resource = transformer(resource)
	}
	return resource
}

// Close implements Fetcher
func (f *TransformingFetcher) Close() {
	f.fetcher.Close()
}

func NewTransformingFetcher(fetcher Fetcher, transformers ...ResourceTransformer) *TransformingFetcher {
	return &TransformingFetcher{
		fetcher:      fetcher,
		transformers: transformers,
	}
}
