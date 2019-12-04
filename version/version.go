package version

// Version components
const (
	Maj = "0"
	Min = "1"
	Fix = "0"
)

var (
	// Version is the current version.
	Version = "0.1.3"

	// GitCommit is the current HEAD set using ldflags.
	GitCommit string
	// GitBranch is the current HEAD set using ldflags.
	GitBranch string
)
