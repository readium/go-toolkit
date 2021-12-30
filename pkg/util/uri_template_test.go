package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUTParameters(t *testing.T) {
	assert.Equal(
		t,
		[]string{"x", "hello", "y", "z", "w"},
		NewURITemplate("/url{?x,hello,y}name{z,y,w}").Parameters(),
	)
}

func TestUTExpandSimpleStringTemplates(t *testing.T) {
	parameters := map[string]string{
		"x":     "aaa",
		"hello": "Hello, world",
		"y":     "b",
		"z":     "45",
		"w":     "w",
	}
	assert.Equal(
		t,
		"/urlaaa,Hello,%20world,bname45,b,w",
		NewURITemplate("/url{x,hello,y}name{z,y,w}").Expand(parameters),
	)
}

func TestUTExpandComplicatedTemplated(t *testing.T) { // form-style ampersand-separated templates
	parameters := map[string]string{
		"x":     "aaa",
		"hello": "Hello, world",
		"y":     "b",
	}
	assert.Equal(
		t,
		"/url?x=aaa&hello=Hello,%20world&y=bname",
		NewURITemplate("/url{?x,hello,y}name").Expand(parameters),
	)

	assert.Equal(
		t,
		"https://lsd-test.edrlab.org/licenses/39ef1ff2-cda2-4219-a26a-d504fbb24c17/renew?end=2020-11-12T16:02:00.000%2B01:00&id=38dfd7ba-a80b-4253-a047-e6aa9c21d6f0&name=Pixel%203a",
		NewURITemplate(
			"https://lsd-test.edrlab.org/licenses/39ef1ff2-cda2-4219-a26a-d504fbb24c17/renew{?end,id,name}",
		).Expand(map[string]string{
			"id":   "38dfd7ba-a80b-4253-a047-e6aa9c21d6f0",
			"name": "Pixel 3a",
			"end":  "2020-11-12T16:02:00.000+01:00",
		}),
	)
}
