package epub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func floatP(f float64) *float64 {
	return &f
}

func TestClockValueFullAndPartialClockValues(t *testing.T) {
	assert.Equal(t, floatP(9003.0), ParseClockValue("02:30:03"))
	assert.Equal(t, floatP(180010.25), ParseClockValue("50:00:10.25"))
	assert.Equal(t, floatP(153.0), ParseClockValue(" 02:33"))
	assert.Equal(t, floatP(10.5), ParseClockValue("00:10.5"))
}

func TestClockValueTimecounts(t *testing.T) {
	assert.Equal(t, floatP(11520.0), ParseClockValue("3.2h"))
	assert.Equal(t, floatP(2700.0), ParseClockValue("45min"))
	assert.Equal(t, floatP(30.0), ParseClockValue(" 30s"))
	assert.Equal(t, floatP(0.005), ParseClockValue("5ms"))
	assert.Equal(t, floatP(12.467), ParseClockValue("12.467"))
}
