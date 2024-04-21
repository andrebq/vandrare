package pattern_test

import (
	"testing"

	"github.com/andrebq/vandrare/internal/pattern"
)

func TestPattern(t *testing.T) {
	input := []string{"abc", "123", "456"}
	if !pattern.Match(input, pattern.Prefix([]string{"abc", "123"}, pattern.Equal("456"))) {
		t.Fatal("Match failed but should pass")
	}
}
