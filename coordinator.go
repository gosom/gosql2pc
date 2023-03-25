package gosql2pc

import (
	"context"
	"errors"
	"fmt"
)

// ErrCommitFailed is returned when a participant fails to commit
var ErrCommitFailed = errors.New("commit failed")

// Params is the input for Do
type Params struct {
	// LogFn is a function that can be used for logging errors. Leave empty for no logging
	LogFn        func(msg string, args ...any)
	Participants []Participant
}

// Do runs the distributed transaction
func Do(ctx context.Context, params Params) error {
	log := getLog(params)
	defer func() {
		for i := range params.Participants {
			if err := params.Participants[i].rollback(); err != nil {
				log("rollback failed: %s", err)
			}
		}
	}()
	// prepare all participants
	for i := range params.Participants {
		if err := params.Participants[i].prepare(ctx); err != nil {
			return err
		}
	}
	// commit all participants
	for i := range params.Participants {
		if err := params.Participants[i].commit(); err != nil {
			log("commit failed: %s", err)
			// since we have committed this participant, we need to rollback all other participants
			// this may leave an inconsistent state
			return fmt.Errorf("%w: %s", ErrCommitFailed, err.Error())
		}
	}
	return nil
}

var defaultLog func(msg string, args ...any) = func(msg string, args ...any) {}
var defaultPrefix = "gosql2pc-"

func getLog(p Params) func(msg string, args ...any) {
	if p.LogFn != nil {
		return p.LogFn
	}
	return defaultLog
}
