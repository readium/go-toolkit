package manifest

// ReadingProgression
// This is not a proper enum replacement! Use the validator to enforce the values
type ReadingProgression string

const (
	Auto ReadingProgression = "auto"
	LTR  ReadingProgression = "ltr"
	RTL  ReadingProgression = "rtl"
	TTB  ReadingProgression = "ttb"
	BTT  ReadingProgression = "btt"
)

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
