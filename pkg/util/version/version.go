package version

import (
	"runtime/debug"
	"time"
)

const toolkitRepo = "github.com/readium/go-toolkit"

var Version = "unknown"

type vcsInfo struct {
	VCS      string
	Revision string
	Time     string
	Modified string
}

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
		if info.Main.Path == toolkitRepo && Version == "unknown" {
			// Try instead using vcs info
			vcs := vcsInfo{}
			for _, v := range info.Settings {
				switch v.Key {
				case "vcs":
					vcs.VCS = v.Value
				case "vcs.revision":
					vcs.Revision = v.Value
				case "vcs.time":
					vcs.Time = v.Value
				case "vcs.modified":
					vcs.Modified = v.Value
				}
			}
			vcsToVersion(vcs)
		}
	}
}

func vcsToVersion(vcs vcsInfo) {
	if vcs.VCS != "git" || vcs.Revision == "" || vcs.Time == "" {
		return
	}

	t, err := time.Parse(time.RFC3339, vcs.Time)
	if err != nil {
		return
	}

	Version = "v0.0.0-" + t.UTC().Format("20060102150405") + "-" + vcs.Revision[:12]
	if vcs.Modified == "true" {
		Version += "+dirty"
	}
}
