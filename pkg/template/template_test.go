package template

import (
	"testing"

	"github.com/stakater/konfigurator/pkg/reconcileerr"
)

type ctx struct{ Pods []string }

// A parse error is the author's to fix: permanent. An execute error depends on
// the data, which is live cluster state that changes without a CR edit: next
// cycle (PR #166 review, round 4).
func TestExecuteStringErrorClassification(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		data      interface{}
		permanent bool
		nextCycle bool
	}{
		{"parse error", "{{ .Pods ", nil, true, false},
		{"execute: index on an empty list", "{{ (index .Pods 0) }}", ctx{}, false, true},
		{"execute: missing field", "{{ .Missing.Field }}", struct{}{}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExecuteString(tt.text, tt.data)
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := reconcileerr.IsPermanent(err); got != tt.permanent {
				t.Fatalf("permanent = %v, want %v: %v", got, tt.permanent, err)
			}
			if got := reconcileerr.IsNextCycle(err); got != tt.nextCycle {
				t.Fatalf("nextCycle = %v, want %v: %v", got, tt.nextCycle, err)
			}
		})
	}
	if out, err := ExecuteString("{{ index .Pods 0 }}", ctx{Pods: []string{"a"}}); err != nil || string(out) != "a" {
		t.Fatalf("valid template: out=%q err=%v", out, err)
	}
}
