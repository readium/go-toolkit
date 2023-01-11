help:
	@echo "Usage: make <target>\n\n\
	  build\t\tBuild the \`rwp\` command-line utility in the current directory\n\
	  install\tBuild and install the \`rwp\` command-line utility\n\
	"

.PHONY: build
build:
	(cd cmd/rwp; go build; mv rwp ../..)

.PHONY: install
install:
	(cd cmd/rwp; go install)


