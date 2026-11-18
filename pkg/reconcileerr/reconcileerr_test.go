package reconcileerr

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassification(t *testing.T) {
	base := errors.New("boom")
	tests := []struct {
		name      string
		err       error
		permanent bool
		nextCycle bool
	}{
		{"nil", nil, false, false},
		{"plain error is transient", base, false, false},
		{"permanent", Permanent(base), true, false},
		{"next cycle", NextCycle(base), false, true},
		{"permanent, then wrapped by the caller", fmt.Errorf("mount: %w", Permanent(base)), true, false},
		{"next cycle, then wrapped by the caller", fmt.Errorf("render: %w", NextCycle(base)), false, true},
		{"wrapped, then marked", Permanent(fmt.Errorf("render: %w", base)), true, false},
		{"transient wrapped", fmt.Errorf("api: %w", base), false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermanent(tt.err); got != tt.permanent {
				t.Fatalf("IsPermanent = %v, want %v", got, tt.permanent)
			}
			if got := IsNextCycle(tt.err); got != tt.nextCycle {
				t.Fatalf("IsNextCycle = %v, want %v", got, tt.nextCycle)
			}
			if tt.err != nil {
				if !errors.Is(tt.err, base) {
					t.Fatal("marking must keep the original error reachable")
				}
				if tt.err.Error() == "" || (tt.permanent && Permanent(base).Error() != base.Error()) {
					t.Fatal("marking must not change the message")
				}
			}
		})
	}
	if Permanent(nil) != nil || NextCycle(nil) != nil {
		t.Fatal("marking nil must stay nil")
	}
}
