package epub

import (
	"strconv"
	"strings"
	"unicode"
)

// Parse clock values as defined in https://www.w3.org/TR/SMIL/smil-timing.html#q2
func ParseClockValue(raw string) *float64 {
	if raw == "" {
		return nil
	}
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, ":") {
		fv := parseClockValue(raw)
		return &fv
	} else {
		metricStart := strings.IndexFunc(raw, func(r rune) bool {
			return unicode.IsLetter(r)
		})
		if metricStart == -1 {
			fval, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return nil
			}
			return parseTimecount(fval, "")
		} else {
			count, err := strconv.ParseFloat(raw[:metricStart], 64)
			if err != nil {
				return nil
			}
			metric := raw[metricStart:]
			return parseTimecount(count, metric)
		}
	}
}

func parseClockValue(raw string) float64 {
	rawParts := strings.Split(raw, ":")
	parts := make([]float64, len(rawParts))
	for i, p := range rawParts {
		fval, err := strconv.ParseFloat(p, 64)
		if err == nil {
			parts[i] = fval
		}
	}
	minSec := parts[len(parts)-1] + parts[len(parts)-2]*60
	if len(parts) > 2 {
		fv := minSec + parts[len(parts)-3]*3600
		return fv
	} else {
		return minSec
	}
}

func parseTimecount(value float64, metric string) *float64 {
	switch metric {
	case "h":
		value *= 3600
		return &value
	case "min":
		value *= 60
		return &value
	case "s":
		fallthrough
	case "":
		return &value
	case "ms":
		value /= 1000
		return &value
	default:
		return nil
	}
}
