package fetcher

import "github.com/readium/go-toolkit/pkg/manifest"

// Transforms the resources' content of a child fetcher using a list of [ResourceTransformer] functions.
type TransformingFetcher struct {
	fetcher      Fetcher
	transformers []ResourceTransformer
}

func (f *TransformingFetcher) Links() ([]manifest.Link, error) {
	return f.fetcher.Links()
}

func (f *TransformingFetcher) Get(link manifest.Link) Resource {
	resource := f.fetcher.Get(link)
	for _, transformer := range f.transformers {
		resource = transformer(resource)
	}
	return resource
}

func (f *TransformingFetcher) Close() {
	f.fetcher.Close()
}

func NewTransformingFetcher(fetcher Fetcher, transformers ...ResourceTransformer) *TransformingFetcher {
	return &TransformingFetcher{
		fetcher:      fetcher,
		transformers: transformers,
	}
}
