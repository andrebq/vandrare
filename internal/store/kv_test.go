package store_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/andrebq/vandrare/internal/store"
)

func TestBasicKV(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	ops := st.Ops(false)
	kv := ops.KV()
	world := []byte("world")
	kv.SetBytes(context.Background(), "hello", world)

	if buf := kv.GetBytes(context.Background(), nil, "hello"); !bytes.Equal(world, buf) {
		t.Fatal("Invalid data", kv.Err(), ops.Err())
	}
	ops.Close()

	ops = st.Ops(true)
	kv = ops.KV()
	if buf := kv.GetBytes(context.Background(), nil, "hello"); bytes.Equal(world, buf) {
		t.Fatal("Previous transaction should have been rolledback, but wasn't")
	}

	kv.SetBytes(context.Background(), "hello", world)
	ops.Close()

	ops = st.Ops(false)
	kv = ops.KV()
	if buf := kv.GetBytes(context.Background(), nil, "hello"); !bytes.Equal(world, buf) {
		t.Fatal("Invalid data after commit")
	}
	ops.Close()
}
