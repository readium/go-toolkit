package parser

// Parser TODO add doc
type Parser interface {
	Parse(filename string, filepath string, host string)
}
