package store_test

import (
	"testing"

	"github.com/andrebq/vandrare/internal/store"
)

func TestInit(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	_ = st
}
