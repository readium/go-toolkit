package manifest

type ReadingProgression string

const (
	None ReadingProgression = ""
	LTR  ReadingProgression = "ltr"
	RTL  ReadingProgression = "rtl"
)

func (r ReadingProgression) correct() ReadingProgression {
	switch r {
	case LTR, RTL:
		return r
	default:
		return None // Default to None if the value is not recognized
	}
}
