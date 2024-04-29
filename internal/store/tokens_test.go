package store_test

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/andrebq/vandrare/internal/store"
)

func TestTokens(t *testing.T) {
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}

	ops := st.Ops(false)
	defer ops.Close()

	tks := ops.Tokens()
	forever, err := tks.Issue(context.Background(), "random-user", "description", -1)
	if err != nil {
		t.Fatal(err)
	}

	expired, err := tks.Issue(context.Background(), "random-user", "expired", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if err := ops.Commit(); err != nil {
		t.Fatal(err)
	}

	ops = st.Ops(false)
	defer ops.Close()
	tks = ops.Tokens()

	if valid, user, err := tks.Valid(context.Background(), (*forever)[:]); err != nil {
		t.Fatal("Should be valid", err)
	} else if !valid {
		t.Fatal("Not valid but without an error!")
	} else if user != "random-user" {
		t.Fatal("User do not match", user)
	}

	if valid, _, err := tks.Valid(context.Background(), (*expired)[:]); err == nil || valid {
		t.Fatal("Should have expired!")
	} else if !errors.Is(err, store.ErrInvalidToken) {
		t.Fatalf("Expecting %v got %v", store.ErrInvalidToken, err)
	}

	var randkey [32]byte
	if _, err := rand.Read(randkey[:]); err != nil {
		t.Fatal(err)
	} else if valid, _, err := tks.Valid(context.Background(), randkey[:]); err == nil || valid {
		t.Fatal("System authorized a random key as valid!")
	}
}
