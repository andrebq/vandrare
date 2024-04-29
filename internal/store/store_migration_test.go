package store

import "testing"

func TestMigration(t *testing.T) {
	st, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	if err := initDB(st.db); err != nil {
		t.Fatal("initializing after opening should be a no-op", err)
	}
}
