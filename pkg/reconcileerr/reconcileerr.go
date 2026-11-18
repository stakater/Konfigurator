// Package reconcileerr classifies errors for the reconciler, at the source.
//
// Three classes, because they need three different reactions:
//
//   - Permanent: only a CR edit or an operator restart can fix it (a template that
//     does not parse, a webhook host the admin has not allow-listed, an invalid
//     spec). Both of those cause a new reconcile anyway, so the reconciler returns
//     reconcile.TerminalError: logged, counted, not requeued. Retrying would only
//     burn the worker.
//   - NextCycle: the error depends on cluster state the operator does not watch
//     (an execute error on a momentarily empty Pods list, a webhook that answers
//     allowed:false for the current content, a container a GitOps sync adds a
//     minute later). Nothing will re-trigger the template, so the reconciler keeps
//     the normal render rhythm: log it and RequeueAfter(updateFrequency). No
//     backoff storm, and the template recovers on its own.
//   - Anything unmarked is transient (timeout, 5xx, API conflict, an app not there
//     yet) and is returned for controller-runtime's backoff.
//
// Producers mark where the knowledge is; the reconciler only asks.
package reconcileerr

import "errors"

type class int

const (
	permanentClass class = iota + 1
	nextCycleClass
)

type marked struct {
	err error
	cls class
}

func (m *marked) Error() string { return m.err.Error() }
func (m *marked) Unwrap() error { return m.err }

// Is matches a probe of the same class anywhere in the chain.
func (m *marked) Is(target error) bool {
	t, ok := target.(*marked)
	return ok && t.cls == m.cls
}

var (
	permanentProbe = &marked{err: errors.New("permanent"), cls: permanentClass}
	nextCycleProbe = &marked{err: errors.New("next cycle"), cls: nextCycleClass}
)

// Permanent marks err as fixable only by a CR edit or restart. nil stays nil; the
// message is unchanged and the original error stays reachable.
func Permanent(err error) error { return mark(err, permanentClass) }

// NextCycle marks err as dependent on cluster state: re-render at the next
// scheduled cycle instead of backing off or giving up.
func NextCycle(err error) error { return mark(err, nextCycleClass) }

func mark(err error, cls class) error {
	if err == nil {
		return nil
	}
	return &marked{err: err, cls: cls}
}

// IsPermanent reports whether err, or anything it wraps, was marked Permanent.
func IsPermanent(err error) bool { return errors.Is(err, permanentProbe) }

// IsNextCycle reports whether err, or anything it wraps, was marked NextCycle.
func IsNextCycle(err error) bool { return errors.Is(err, nextCycleProbe) }
