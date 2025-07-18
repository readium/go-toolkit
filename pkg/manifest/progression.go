package manifest

type ReadingProgression string

const (
	Auto ReadingProgression = "auto"
	LTR  ReadingProgression = "ltr"
	RTL  ReadingProgression = "rtl"
	TTB  ReadingProgression = "ttb"
	BTT  ReadingProgression = "btt"
)

func (r ReadingProgression) Correct() ReadingProgression {
	switch r {
	case Auto, LTR, RTL, TTB, BTT:
		return r
	default:
		return Auto // Default to Auto if the value is not recognized
	}
}

func IsHorizontal(progression ReadingProgression) *bool {
	switch progression {
	case LTR, RTL:
		return newBool(true)
	case TTB, BTT:
		return newBool(false)
	default:
		return nil
	}
}
