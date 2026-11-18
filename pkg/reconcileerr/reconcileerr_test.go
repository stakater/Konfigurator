package reconcileerr

import (
	"errors"
	"fmt"
	"testing"
)

func TestPermanent(t *testing.T) {
	base := errors.New("boom")
	tests := []struct {
		name      string
		err       error
		permanent bool
	}{
		{"nil", nil, false},
		{"plain error", base, false},
		{"marked", Permanent(base), true},
		{"marked then wrapped by the caller", fmt.Errorf("mount failed: %w", Permanent(base)), true},
		{"wrapped then marked", Permanent(fmt.Errorf("render: %w", base)), true},
		{"transient wrapped next to nothing permanent", fmt.Errorf("api: %w", base), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermanent(tt.err); got != tt.permanent {
				t.Fatalf("IsPermanent = %v, want %v", got, tt.permanent)
			}
			if tt.err != nil {
				if !errors.Is(tt.err, base) {
					t.Fatal("marking must keep the original error reachable")
				}
				if Permanent(base).Error() != base.Error() {
					t.Fatal("marking must not change the message")
				}
			}
		})
	}
	if Permanent(nil) != nil {
		t.Fatal("Permanent(nil) must be nil")
	}
}
