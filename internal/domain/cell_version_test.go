package domain

import (
	"math"
	"testing"
)

func TestNewCellVersionは1以上の値だけを受け付ける(t *testing.T) {
	for _, value := range []uint64{1, 2, math.MaxUint64} {
		version, err := NewCellVersion(value)
		if err != nil {
			t.Fatalf("NewCellVersion(%d) returned %v", value, err)
		}
		if uint64(version) != value {
			t.Fatalf("version = %d, want %d", version, value)
		}
	}
	if _, err := NewCellVersion(0); err == nil {
		t.Fatal("version 0 must be rejected")
	}
}
