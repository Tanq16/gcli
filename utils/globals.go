package utils

var GlobalDebugFlag bool

var GlobalForAIFlag bool

// Exit-code vocabulary (spec §3.1/§9.4). PrintFatal derives the code from an
// error implementing ExitCode() int; PrintFatalCode sets it explicitly.
const (
	ExitGeneric     = 1
	ExitUsage       = 2
	ExitAuth        = 3
	ExitNotFound    = 4
	ExitPermission  = 5
	ExitPartial     = 6
	ExitRateLimited = 7
	ExitCancelled   = 130
)
