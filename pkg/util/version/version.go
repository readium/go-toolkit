package version

import "runtime/debug"

const toolkitRepo = "github.com/readium/go-toolkit"

var Version = "unknown"

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Path == toolkitRepo && info.Main.Version != "(devel)" {
			// This is the toolkit itself
			Version = info.Main.Version
		} else {
			// This is a module that uses the toolkit
			for _, dep := range info.Deps {
				if dep.Path == toolkitRepo {
					Version = dep.Version
					break
				}
			}
		}
	}
}
