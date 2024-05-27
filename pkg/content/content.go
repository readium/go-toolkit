package content

import (
	"strings"

	"github.com/readium/go-toolkit/pkg/content/element"
	"github.com/readium/go-toolkit/pkg/content/iterator"
)

type Content interface {
	Text(separator *string) (string, error) // Extracts the full raw text, or returns null if no text content can be found.
	Iterator() iterator.Iterator            // Creates a new iterator for this content.
	Elements() ([]element.Element, error)   // Returns all the elements as a list.
}

// Extracts the full raw text, or returns null if no text content can be found.
func ContentText(content Content, separator *string) (string, error) {
	sep := "\n"
	if separator != nil {
		sep = *separator
	}
	var sb strings.Builder
	els, err := content.Elements()
	if err != nil {
		return "", err
	}
	for _, el := range els {
		if txel, ok := el.(element.TextualElement); ok {
			txt := txel.Text()
			if txt != "" {
				sb.WriteString(txel.Text())
				sb.WriteString(sep)
			}
		}
	}
	return strings.TrimSuffix(sb.String(), sep), nil
}

func ContentElements(content Content) ([]element.Element, error) {
	var elements []element.Element
	it := content.Iterator()
	for {
		hasNext, err := it.HasNext()
		if err != nil {
			return nil, err
		}
		if !hasNext {
			break
		}
		elements = append(elements, it.Next())
	}
	return elements, nil
}
