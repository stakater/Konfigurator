// Package webhook calls the user-configured template validation webhook, bounded
// and restricted to admin-approved hosts (review S2, story H.2).
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/stakater/konfigurator/pkg/reconcileerr"
)

const (
	// AllowedHostsEnv is the operator env var holding the comma-separated allow-list.
	AllowedHostsEnv = "VALIDATION_WEBHOOK_ALLOWED_HOSTS"

	DefaultTimeout          = 10 * time.Second
	DefaultMaxResponseBytes = 64 << 10
)

var errRedirect = errors.New("redirects are not allowed")

// Validator posts rendered templates to a validation webhook. The zero value (or a
// nil *Validator) denies every call: webhooks must be explicitly enabled by the
// operator admin through AllowedHosts.
type Validator struct {
	// AllowedHosts are matched case-insensitively against the URL. A bare host
	// ("svc.ns.svc") allows that host on any port; a host:port entry
	// ("svc.ns.svc:8443") allows only that port. Empty disables webhooks.
	AllowedHosts     []string
	Timeout          time.Duration
	MaxResponseBytes int64
	// Client is optional; when nil one is built from Timeout that refuses redirects.
	Client *http.Client
}

type response struct {
	Allowed bool   `json:"allowed"`
	Message string `json:"message"`
}

// Validate sends rendered to rawURL and returns nil only if the webhook answered
// 2xx with {"allowed": true}. Nothing is sent unless the URL is http(s) and its
// host is allow-listed.
//
// Errors are classified for the reconciler (pkg/reconcileerr): configuration
// failures only a CR edit or the admin can fix (bad URL or scheme, host not
// allowed, redirect) are Permanent; failures that depend on the rendered content
// or on the server's reply right now (allowed:false, an oversized or non-JSON 2xx
// such as a proxy maintenance page) are NextCycle; network failures and non-2xx
// replies are left plain so they are retried with backoff.
func (v *Validator) Validate(ctx context.Context, rawURL string, rendered map[string]string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return reconcileerr.Permanent(fmt.Errorf("invalid webhook URL %q: %w", rawURL, err))
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return reconcileerr.Permanent(fmt.Errorf("webhook URL %q: unsupported scheme %q (want http or https)", rawURL, u.Scheme))
	}
	if v == nil || len(v.AllowedHosts) == 0 {
		return reconcileerr.Permanent(fmt.Errorf("validation webhooks are disabled: no allowed hosts configured by the operator admin (%s)", AllowedHostsEnv))
	}
	if !v.allowed(u) {
		return reconcileerr.Permanent(fmt.Errorf("webhook host %q is not in the allowed hosts configured by the operator admin (%s)", u.Host, AllowedHostsEnv))
	}

	body, err := json.Marshal(rendered)
	if err != nil {
		return reconcileerr.Permanent(fmt.Errorf("webhook request serialization failed: %w", err))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client().Do(req)
	if err != nil {
		err = fmt.Errorf("webhook request failed: %w", err)
		if errors.Is(err, errRedirect) {
			return reconcileerr.Permanent(err) // the webhook is misconfigured, not flaky
		}
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	limit := v.MaxResponseBytes
	if limit <= 0 {
		limit = DefaultMaxResponseBytes
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return fmt.Errorf("webhook response read failed: %w", err)
	}
	if int64(len(raw)) > limit {
		return reconcileerr.NextCycle(fmt.Errorf("webhook response exceeds %d bytes", limit))
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	var res response
	if err := json.Unmarshal(raw, &res); err != nil {
		return reconcileerr.NextCycle(fmt.Errorf("webhook response decoding failed: %w", err))
	}
	if !res.Allowed {
		return reconcileerr.NextCycle(fmt.Errorf("template is invalid: %s", res.Message))
	}
	return nil
}

func (v *Validator) allowed(u *url.URL) bool {
	host, hostPort := strings.ToLower(u.Hostname()), strings.ToLower(u.Host)
	for _, a := range v.AllowedHosts {
		a = strings.ToLower(a)
		if a != "" && (a == host || a == hostPort) {
			return true
		}
	}
	return false
}

func (v *Validator) client() *http.Client {
	if v.Client != nil {
		return v.Client
	}
	timeout := v.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &http.Client{
		Timeout: timeout,
		// An allowed host must not be able to bounce the request elsewhere.
		CheckRedirect: func(*http.Request, []*http.Request) error { return errRedirect },
	}
}

// ParseAllowedHosts splits a comma-separated allow-list, trimming blanks and
// lower-casing entries.
func ParseAllowedHosts(csv string) []string {
	var out []string
	for _, h := range strings.Split(csv, ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			out = append(out, h)
		}
	}
	return out
}
