package epub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEPUBPositionsServiceEmptyReadingOrder(t *testing.T) {
	service := PositionsService{}
	assert.Equal(t, 0, len(service.Positions()))
}

// TODO replicate `createService` tester from Kotlin

/*func TestEPUBPositionsServiceSingleReadingOrder(t *testing.T) {
	service := EPUBPositionsService{
		readingOrder: manifest.LinkList{{Href: "res", Type: "application/xml"}},
	}

	assert.Equal(t, []manifest.Locator{{
		Href: "res",
		Type: "application/xml",
		Locations: &manifest.Locations{
			Progression:      0.0,
			Position:         1,
			TotalProgression: 0.0,
		},
	}}, service.Positions())
}
*/
