package main

import (
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
	"github.com/andrewrong/optdesk/internal/store/jobrequestpg"
)

// Design: docs/design/job-console.md#Decisions 1 and 9. The three openers of the request queue travel as one
// value in the ext deps of the app, so that wire.go holds nothing of the console.

// jobConsoleExtKey is the name the openers are registered under in the ext deps.
const jobConsoleExtKey = "jobconsole"

// jobRequestOpeners are the openers of the three roles of the request queue: the pages role reads, the web role
// posts and withdraws, the runner role consumes.
type jobRequestOpeners struct {
	Reader   jobrequest.OpenReaderFunc
	Poster   jobrequest.OpenPosterFunc
	Consumer jobrequest.OpenConsumerFunc
}

// defaultJobRequestOpeners opens the stores on PostgreSQL.
func defaultJobRequestOpeners() jobRequestOpeners {
	return jobRequestOpeners{Reader: jobrequestpg.OpenReader, Poster: jobrequestpg.OpenPoster, Consumer: jobrequestpg.OpenConsumer}
}

func init() {
	extDeps[jobConsoleExtKey] = func() any { return defaultJobRequestOpeners() }
}

// jobRequestOpenersOf returns the openers of d; false when the ext deps hold none (a test literal of deps), in
// which case the console is off.
func jobRequestOpenersOf(d deps) (jobRequestOpeners, bool) {
	o, ok := d.ext[jobConsoleExtKey].(jobRequestOpeners)
	return o, ok
}
