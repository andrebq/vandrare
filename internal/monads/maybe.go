package monads

type (
	Maybe[T any] struct {
		v     T
		valid bool
	}
)

func Must[T any](m Maybe[T]) T {
	var zero T
	if !m.Get(&zero) {
		panic("not set ")
	}
	return zero
}

func Default[T any](m Maybe[T], fallback func() T) T {
	var zero T
	if !m.Get(&zero) {
		return fallback()
	}
	return zero
}

func Self[T any](v T) func() T { return func() T { return v } }

func Some[T any](val T) Maybe[T] {
	return Maybe[T]{v: val}
}

func Nothing[T any]() Maybe[T] {
	return Maybe[T]{}
}

func (s Maybe[T]) Valid() bool { return s.valid }
func (s Maybe[T]) Get(out *T) bool {
	if s.valid {
		*out = s.v
	}
	return s.valid
}
