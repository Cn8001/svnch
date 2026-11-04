package svn

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

var ErrInvalidRequest = errors.New("invalid request")

type Depth string

const (
	DepthDefault    Depth = ""
	DepthEmpty      Depth = "empty"
	DepthFiles      Depth = "files"
	DepthImmediates Depth = "immediates"
	DepthInfinity   Depth = "infinity"
)

type CheckoutRequest struct {
	URL      string `json:"url"`
	Path     string `json:"path"`
	Revision int64  `json:"revision"` // 0 means HEAD
	Depth    Depth  `json:"depth"`
}

func CheckoutArgs(req CheckoutRequest) ([]string, error) {
	if req.URL == "" || req.Path == "" {
		return nil, fmt.Errorf("checkout needs a url and a path: %w", ErrInvalidRequest)
	}
	if req.Revision < 0 {
		return nil, fmt.Errorf("checkout revision %d: %w", req.Revision, ErrInvalidRequest)
	}

	var args []string
	if req.Revision > 0 {
		args = append(args, "-r", strconv.FormatInt(req.Revision, 10))
	}
	switch req.Depth {
	case DepthDefault:
	case DepthEmpty, DepthFiles, DepthImmediates, DepthInfinity:
		args = append(args, "--depth", string(req.Depth))
	default:
		return nil, fmt.Errorf("checkout depth %q: %w", req.Depth, ErrInvalidRequest)
	}
	return append(args, "--", pegSafe(req.URL), pegSafe(req.Path)), nil
}

// ParseCheckout reads the revision from the closing line, since checkout has no --xml output.
func ParseCheckout(out []byte) (int64, error) {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range slices.Backward(lines) {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "Checked out revision ")
		if !ok {
			continue
		}
		rev, err := strconv.ParseInt(strings.TrimSuffix(rest, "."), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse checkout revision %q: %w", rest, err)
		}
		return rev, nil
	}
	return 0, errors.New("parse checkout: no revision line in output")
}
