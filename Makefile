help:
	@echo "Usage: make <target>\n\n\
	  build\tBuild the `readium` command-line utility in the current directory\n\
	  install\tBuild and install the `readium` command-line utility\n\
	"

.PHONY: build
build:
	(cd cmd/readium; go build; mv readium ../..)

.PHONY: install
install:
	(cd cmd/readium; go install)


