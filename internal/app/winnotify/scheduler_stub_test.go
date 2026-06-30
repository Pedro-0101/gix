//go:build !windows

package winnotify

import (
	"testing"
	"time"
)

func TestStubIsNoop(t *testing.T) {
	n := New()
	if err := n.Arm(Occurrence{AlertID: 1, FireAt: time.Now(), Message: "x"}); err != nil {
		t.Fatalf("Arm: %v", err)
	}
	if err := n.CancelByAlert(1); err != nil {
		t.Fatalf("CancelByAlert: %v", err)
	}
	got, err := n.ListArmed()
	if err != nil {
		t.Fatalf("ListArmed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListArmed = %v, quero vazio", got)
	}
}
