package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Cn8001/svnch/internal/servers"
	"github.com/Cn8001/svnch/internal/svn"
)

// Failure is every error as the webview sees it: a stable code to branch on and the message to show.
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	SvnCode string `json:"svnCode,omitempty"` // E155007 and the like, when svn named one
}

func (f *Failure) Error() string { return f.Message }

func code(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "CANCELLED"
	case errors.Is(err, context.DeadlineExceeded):
		return "TIMEOUT"
	case errors.Is(err, svn.ErrSvnNotFound):
		return "SVN_MISSING"
	case errors.Is(err, svn.ErrAuthFailed):
		return "AUTH"
	case errors.Is(err, svn.ErrCertUntrusted):
		return "CERT"
	case errors.Is(err, svn.ErrNotWorkingCopy):
		return "NOT_WORKING_COPY"
	case errors.Is(err, svn.ErrNeedsCleanup):
		return "NEEDS_CLEANUP"
	case errors.Is(err, svn.ErrInvalidRequest), errors.Is(err, servers.ErrInvalid):
		return "INVALID_REQUEST"
	case errors.Is(err, ErrUpdatesOff):
		return "UPDATES_OFF"
	}
	return "INTERNAL"
}

func ToFailure(err error) *Failure {
	if err == nil {
		return nil
	}
	var f *Failure
	if errors.As(err, &f) {
		return f
	}
	out := &Failure{Code: code(err), Message: err.Error()}
	var ce *svn.CommandError
	if errors.As(err, &ce) {
		out.SvnCode = ce.Code
	}
	return out
}

// MarshalError is wired into the Wails options, so a bound method returns a plain wrapped error and the webview
// still receives a Failure as its cause.
func MarshalError(err error) []byte {
	b, mErr := json.Marshal(ToFailure(err))
	if mErr != nil {
		return nil
	}
	return b
}
