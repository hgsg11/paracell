package domain

import (
	"math"
	"testing"
)

func TestCellVersionAddはNewCellVersionを通して新しい値を返す(t *testing.T) {
	version, err := NewCellVersion(1)
	if err != nil {
		t.Fatal(err)
	}

	added, err := version.Add()
	if err != nil {
		t.Fatal(err)
	}
	if version != 1 || added != 2 {
		t.Fatalf("version = %d, added = %d", version, added)
	}

	maximum, err := NewCellVersion(math.MaxUint64)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maximum.Add(); err == nil {
		t.Fatal("overflowed version must be rejected")
	}
}
