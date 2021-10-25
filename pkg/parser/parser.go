package parser

import "github.com/readium/go-toolkit/pkg/pub"

type PublicationParser interface {
	Parse() (*pub.Builder, error)
}
