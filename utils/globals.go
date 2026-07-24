package utils

// GlobalDebugFlag is set by cobra root command when --debug is passed
var GlobalDebugFlag bool

// GlobalForAIFlag is set by cobra root command when --for-ai is passed
// When true, output uses plain text with prefixes and input reads from stdin pipe
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
