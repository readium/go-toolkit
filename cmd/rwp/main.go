package main

import (
	"os"

	"github.com/readium/go-toolkit/cmd/rwp/cmd"
)

func main() {
	// From the archive/zip docs:
	// If any file inside the archive uses a non-local name
	// (as defined by [filepath.IsLocal]) or a name containing backslashes
	// and the GODEBUG environment variable contains `zipinsecurepath=0`,
	// NewReader returns the reader with an [ErrInsecurePath] error.
	if os.Getenv("GODEBUG") == "" {
		os.Setenv("GODEBUG", "zipinsecurepath=0")
	} else {
		os.Setenv("GODEBUG", os.Getenv("GODEBUG")+",zipinsecurepath=0")
	}

	cmd.Execute()
}
