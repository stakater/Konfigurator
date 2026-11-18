// Package reconcileerr marks errors that a reconcile cannot fix by retrying.
//
// A permanent error (a template that does not parse, a webhook the admin has not
// allow-listed, a container that does not exist) stays wrong until a human changes
// something. Retrying it only burns the controller's worker and fills the log, so
// the reconciler turns marked errors into reconcile.TerminalError. Everything else
// (timeouts, 5xx, API conflicts, an app that is not there yet) is retried with
// backoff. Producers mark at the source, where the knowledge is; the reconciler
// only asks IsPermanent.
package reconcileerr

import "errors"

type permanent struct{ err error }

func (p *permanent) Error() string { return p.err.Error() }
func (p *permanent) Unwrap() error { return p.err }

// Is lets errors.Is(err, marker) match any permanent in the chain.
func (p *permanent) Is(target error) bool { _, ok := target.(*permanent); return ok }

var marker = &permanent{err: errors.New("permanent")}

// Permanent marks err as not worth retrying. The message is unchanged, and the
// original error stays reachable through errors.Is / errors.As. nil stays nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanent{err: err}
}

// IsPermanent reports whether err, or anything it wraps, was marked Permanent.
func IsPermanent(err error) bool { return errors.Is(err, marker) }
