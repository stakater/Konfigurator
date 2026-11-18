package template

import (
	"testing"

	"github.com/stakater/konfigurator/pkg/reconcileerr"
)

func TestExecuteStringErrorsArePermanent(t *testing.T) {
	tests := []struct {
		name string
		text string
		data interface{}
	}{
		{"parse error", "{{ .Pods ", nil},
		{"execute error", "{{ .Missing.Field }}", struct{}{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExecuteString(tt.text, tt.data)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !reconcileerr.IsPermanent(err) {
				t.Fatalf("a template error is the author's to fix, must be permanent: %v", err)
			}
		})
	}
	if _, err := ExecuteString("ok {{ . }}", "x"); err != nil {
		t.Fatalf("valid template: %v", err)
	}
}
