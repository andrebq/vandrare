package stack_test

import (
	"testing"

	"github.com/andrebq/vandrare/internal/stack"
)

func TestStack(t *testing.T) {
	st := stack.S[int]{}
	st.Push(1)
	st.Push(2)
	st.Push(3)
	st.Push(4)

	if st.Peek() != 4 {
		t.Fatal("stack order not hold")
	}

	if val := st.Pop(); val != 4 {
		t.Fatal("stack order not hold before discard")
	}

	if !st.Discard(2) {
		t.Fatal("missing item from stack")
	}

	if val := st.Pop(); val != 3 {
		t.Fatal("stack order not hold after discard")
	}

	st.Pop()

	if !st.Empty() {
		t.Fatal("stack not empty")
	}
}
