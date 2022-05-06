package main

import "github.com/omnibuildplatform/omni-orchestrator/cmd"

var (
	Tag       string //Git tag name, filled when generating binary
	CommitID  string //Git commit ID, filled when generating binary
	ReleaseAt string //Publish date, filled when generating binary
)

func main() {
	cmd.Execute(Tag, CommitID, ReleaseAt)
}
