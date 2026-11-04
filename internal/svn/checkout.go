package svn

import (
	"errors"
	"fmt"
	"strconv"
)

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
	t, err := targets(req.URL, req.Path)
	if err != nil {
		return nil, err
	}
	return append(args, t...), nil
}

func ParseCheckout(out []byte) (int64, error) {
	rev, ok, err := lastRevision(out, "Checked out revision ")
	if err != nil {
		return 0, fmt.Errorf("parse checkout: %w", err)
	}
	if !ok {
		return 0, errors.New("parse checkout: no revision line in output")
	}
	return rev, nil
}
