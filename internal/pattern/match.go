package pattern

type (
	Matcher[T any, E ~[]T] interface {
		Match(input E) (bool, E)
	}

	equalityMatcher[T comparable, E ~[]T] struct {
		expected T
	}

	all[T any, E ~[]T] struct {
		matchers []Matcher[T, E]
	}
)

func (em equalityMatcher[T, E]) Match(input E) (bool, E) {
	switch {
	case len(input) == 0:
		return false, input
	default:
		equal := input[0] == em.expected
		if !equal {
			return false, input
		}
		return true, input[1:]
	}
}

func (m all[T, E]) Match(input E) (bool, E) {
	valid := true
	var matches int
	for _, m := range m.matchers {
		var match bool
		match, input = m.Match(input)
		if match {
			matches++
		}
		valid = valid && match
		if !valid || len(input) == 0 {
			break
		}
	}
	return valid && len(input) == 0 && matches == len(m.matchers), nil
}

func All[T any, E ~[]T](entries ...Matcher[T, E]) Matcher[T, E] {
	return all[T, E]{matchers: entries}
}

func Equal[T comparable](expected T) Matcher[T, []T] {
	return equalityMatcher[T, []T]{expected: expected}
}

func Prefix[T comparable](prefix []T, tail Matcher[T, []T]) Matcher[T, []T] {
	head := make([]Matcher[T, []T], len(prefix))
	for i, v := range prefix {
		head[i] = Equal(v)
	}
	if tail != nil {
		head = append(head, tail)
	}
	return All[T, []T](head...)
}

func Match[T any](input []T, matchers ...Matcher[T, []T]) bool {
	if len(matchers) == 1 {
		m, _ := matchers[0].Match(input)
		return m
	}
	m, _ := All[T](matchers...).Match(input)
	return m
}
