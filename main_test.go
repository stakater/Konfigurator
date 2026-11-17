package main

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

// The binary builds its manager options through managerOptions; this test asserts
// on that same function (CLAUDE.md: tests build what the binary builds).
func TestManagerOptions(t *testing.T) {
	tests := []struct {
		name           string
		cfg            managerConfig
		wantNamespaces []string // nil = cluster-wide (no DefaultNamespaces)
	}{
		{
			name: "cluster-wide",
			cfg:  managerConfig{MetricsAddr: ":8080", ProbeAddr: ":8081", LeaderElection: true},
		},
		{
			name:           "single namespace from WATCH_NAMESPACE",
			cfg:            managerConfig{MetricsAddr: ":9090", ProbeAddr: ":9091", WatchNamespace: "konfigurator"},
			wantNamespaces: []string{"konfigurator"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			opts := managerOptions(scheme, tt.cfg)

			if opts.Scheme != scheme {
				t.Error("scheme not passed through")
			}
			if opts.Metrics.BindAddress != tt.cfg.MetricsAddr {
				t.Errorf("metrics addr = %q, want %q", opts.Metrics.BindAddress, tt.cfg.MetricsAddr)
			}
			if opts.HealthProbeBindAddress != tt.cfg.ProbeAddr {
				t.Errorf("probe addr = %q, want %q", opts.HealthProbeBindAddress, tt.cfg.ProbeAddr)
			}
			if opts.LeaderElection != tt.cfg.LeaderElection || opts.LeaderElectionID != leaderElectionID {
				t.Errorf("leader election = %v/%q", opts.LeaderElection, opts.LeaderElectionID)
			}
			// Review B3: a reconciler panic must fail one reconcile, never the process.
			if opts.Controller.RecoverPanic == nil || !*opts.Controller.RecoverPanic {
				t.Error("RecoverPanic must be enabled")
			}
			var got []string
			for ns := range opts.Cache.DefaultNamespaces {
				got = append(got, ns)
			}
			if len(got) != len(tt.wantNamespaces) || (len(got) == 1 && got[0] != tt.wantNamespaces[0]) {
				t.Errorf("cache namespaces = %v, want %v", got, tt.wantNamespaces)
			}
		})
	}
}
