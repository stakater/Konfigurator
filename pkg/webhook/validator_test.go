package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type server struct {
	*httptest.Server
	hits int32
}

func newServer(t *testing.T, h http.HandlerFunc) *server {
	t.Helper()
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&s.hits, 1)
		h(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *server) host(t *testing.T) string {
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func reply(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func TestValidate(t *testing.T) {
	rendered := map[string]string{"fluent.conf": "<source/>"}

	tests := []struct {
		name string
		// setup returns the URL to call, the allow-list, and the servers whose
		// hit counts must match wantHits.
		setup    func(t *testing.T) (target string, allowed []string, watched []*server)
		ctx      func() context.Context
		wantErr  string // substring; "" means success
		wantHits []int32
	}{
		{
			name: "allowed host, allowed:true",
			setup: func(t *testing.T) (string, []string, []*server) {
				s := newServer(t, func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					reply(`{"allowed":true}`)(w, r)
				})
				return s.URL + "/validate", []string{s.host(t)}, []*server{s}
			},
			wantHits: []int32{1},
		},
		{
			name: "allowed host, allowed:false carries message",
			setup: func(t *testing.T) (string, []string, []*server) {
				s := newServer(t, reply(`{"allowed":false,"message":"bad regex"}`))
				return s.URL, []string{s.host(t)}, []*server{s}
			},
			wantErr: "bad regex", wantHits: []int32{1},
		},
		{
			name: "host not on allow-list is never contacted",
			setup: func(t *testing.T) (string, []string, []*server) {
				s := newServer(t, reply(`{"allowed":true}`))
				return s.URL, []string{"validator.example.svc"}, []*server{s}
			},
			wantErr: "not in the allowed", wantHits: []int32{0},
		},
		{
			name: "empty allow-list disables webhooks",
			setup: func(t *testing.T) (string, []string, []*server) {
				s := newServer(t, reply(`{"allowed":true}`))
				return s.URL, nil, []*server{s}
			},
			wantErr: "disabled", wantHits: []int32{0},
		},
		{
			name: "allow-list match is case-insensitive",
			setup: func(t *testing.T) (string, []string, []*server) {
				s := newServer(t, reply(`{"allowed":true}`))
				return s.URL, []string{strings.ToUpper(s.host(t))}, []*server{s}
			},
			wantHits: []int32{1},
		},
		{
			name: "bare host entry matches any port",
			setup: func(t *testing.T) (string, []string, []*server) {
				s := newServer(t, reply(`{"allowed":true}`))
				u, _ := url.Parse(s.URL)
				return s.URL, []string{u.Hostname()}, []*server{s}
			},
			wantHits: []int32{1},
		},
		{
			name: "host:port entry pins the port",
			setup: func(t *testing.T) (string, []string, []*server) {
				s := newServer(t, reply(`{"allowed":true}`))
				u, _ := url.Parse(s.URL)
				return s.URL, []string{u.Hostname() + ":1"}, []*server{s}
			},
			wantErr: "not in the allowed", wantHits: []int32{0},
		},
		{
			name:    "file scheme rejected",
			setup:   func(t *testing.T) (string, []string, []*server) { return "file:///etc/passwd", []string{""}, nil },
			wantErr: "scheme",
		},
		{
			name:    "unparsable URL rejected",
			setup:   func(t *testing.T) (string, []string, []*server) { return "::not a url", []string{"x"}, nil },
			wantErr: "invalid",
		},
		{
			name: "slow server times out",
			setup: func(t *testing.T) (string, []string, []*server) {
				s := newServer(t, func(w http.ResponseWriter, r *http.Request) {
					select {
					case <-time.After(600 * time.Millisecond): // > 200ms client timeout
					case <-r.Context().Done():
					}
				})
				return s.URL, []string{s.host(t)}, []*server{s}
			},
			wantErr: "request failed", wantHits: []int32{1},
		},
		{
			name: "oversized response rejected",
			setup: func(t *testing.T) (string, []string, []*server) {
				s := newServer(t, reply(`{"allowed":true,"message":"`+strings.Repeat("x", 2048)+`"}`))
				return s.URL, []string{s.host(t)}, []*server{s}
			},
			wantErr: "exceeds", wantHits: []int32{1},
		},
		{
			name: "redirect is refused and target never contacted",
			setup: func(t *testing.T) (string, []string, []*server) {
				target := newServer(t, reply(`{"allowed":true}`))
				redirector := newServer(t, func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, target.URL, http.StatusFound)
				})
				return redirector.URL, []string{redirector.host(t), target.host(t)}, []*server{redirector, target}
			},
			wantErr: "redirect", wantHits: []int32{1, 0},
		},
		{
			name: "non-2xx is an error",
			setup: func(t *testing.T) (string, []string, []*server) {
				s := newServer(t, func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"allowed":true}`))
				})
				return s.URL, []string{s.host(t)}, []*server{s}
			},
			wantErr: "500", wantHits: []int32{1},
		},
		{
			name: "cancelled context",
			setup: func(t *testing.T) (string, []string, []*server) {
				s := newServer(t, reply(`{"allowed":true}`))
				return s.URL, []string{s.host(t)}, []*server{s}
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: "request failed", wantHits: []int32{0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, allowed, watched := tt.setup(t)
			v := &Validator{AllowedHosts: allowed, Timeout: 200 * time.Millisecond, MaxResponseBytes: 1024}
			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx()
			}

			start := time.Now()
			err := v.Validate(ctx, target, rendered)
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Fatalf("Validate took %v: not bounded by the timeout", elapsed)
			}

			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("want error containing %q, got nil", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
			for i, s := range watched {
				if got := atomic.LoadInt32(&s.hits); got != tt.wantHits[i] {
					t.Errorf("server %d hits = %d, want %d", i, got, tt.wantHits[i])
				}
			}
		})
	}
}

func TestNilValidatorDenies(t *testing.T) {
	var v *Validator
	if err := v.Validate(context.Background(), "http://example.com", nil); err == nil {
		t.Fatal("nil validator must deny")
	}
}

func TestParseAllowedHosts(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"a.svc", []string{"a.svc"}},
		{" a.svc , B.svc:8443,, ", []string{"a.svc", "b.svc:8443"}},
	}
	for _, tt := range tests {
		if got := ParseAllowedHosts(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseAllowedHosts(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
