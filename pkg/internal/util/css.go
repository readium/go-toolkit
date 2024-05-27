package util

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/agext/regexp"
	"github.com/andybalholm/cascadia"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func escapeChar(r rune) string {
	return fmt.Sprintf("\\%x ", int(r))
}

var nonIdentifier = regexp.MustCompile("[^a-zA-Z0-9_-]")
var extraSpace = regexp.MustCompile("\\s+")

// Note - this is a rudimentary implementation
func escapeCSSIdentifier(input string) string {
	if len(input) == 0 {
		return ""
	}

	// Matches CSS non-identifier characters
	input = nonIdentifier.ReplaceAllStringFunc(input, func(match string) string {
		return escapeChar([]rune(match)[0])
	})

	// If identifier starts with a digit, hyphen + digit or two hyphens, escape it
	firstChar := []rune(input)[0]
	if firstChar == '-' {
		if len(input) > 1 && (input[1] == '-' || (input[1] >= '0' && input[1] <= '9')) {
			input = escapeChar(firstChar) + input[1:]
		}
	} else if firstChar >= '0' && firstChar <= '9' {
		input = escapeChar(firstChar) + input[1:]
	}

	return input
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// Get a CSS selector that will uniquely select a provided HTML element
// If the element has an ID, returns #id;
// otherwise returns the parent (if any) CSS selector, followed by '>'
// followed by a unique selector for the element (tag.class.class:nth-child(n)).
// Logic copied from JSoup: https://github.com/jhy/jsoup/blob/0b10d516ed8f907f8fb4acb9a0806137a8988d45/src/main/java/org/jsoup/nodes/Element.java#L829
func CSSSelector(n *html.Node) string {
	if n == nil || n.Type != html.ElementNode {
		return ""
	}

	id := getAttr(n, "id")
	if id != "" {
		// We're making the big assumption that ID is unique, as would be in good HTML
		// TODO investigate if we can assume this in all EPUBs
		return "#" + escapeCSSIdentifier(id)
	}

	var selector strings.Builder
	selector.WriteString(
		// Escape tagname
		escapeCSSIdentifier(n.Data),
	)

	/*
		NOT IMPLEMENTED
		Translate HTML namespace ns:tag to CSS namespace syntax ns|tag
		// escapeCSSIdentifier(n.Namespace) + "|" + the tag name
	*/

	// Add CSS classes to selector
	classNames := extraSpace.Split(getAttr(n, "class"), -1)
	for _, className := range classNames {
		if className == "" {
			continue
		}
		selector.WriteRune('.')
		selector.WriteString(escapeCSSIdentifier(className))
	}

	if n.Parent == nil {
		// No parent, we're done
		return selector.String()
	}

	if n.Parent.Type == html.ElementNode && n.Parent.DataAtom == atom.Html {
		// Parent is the root element, we're done
		return selector.String()
	}

	s, err := cascadia.Parse(selector.String())
	if err != nil {
		panic(errors.Wrap(err, "failed parsing generated CSS selector"))
	}
	if nodes := cascadia.QueryAll(n.Parent, s); len(nodes) > 1 {
		// Figure out the index of this node among its siblings
		idx := 1
		for ps := n.PrevSibling; ps != nil; ps = ps.PrevSibling {
			if ps.Type == html.ElementNode {
				idx++
			}
		}
		selector.WriteString(":nth-child(")
		selector.WriteString(strconv.Itoa(idx))
		selector.WriteRune(')')
	}

	return CSSSelector(n.Parent) + " > " + selector.String()
}
