package set

import (
	"math/rand"
)

type (
	rndset[T comparable] struct {
		items []T
		rnd   *rand.Rand
	}

	Set[T comparable] interface {
		Add(T) bool
		Remove(T) bool
		Len() int
	}

	RandomSet[T comparable] interface {
		Set[T]
		Pick() (T, bool)
	}
)

func Random[T comparable](seed int64) RandomSet[T] {
	return &rndset[T]{
		rnd: rand.New(rand.NewSource(seed)),
	}
}

func (r *rndset[T]) Len() int { return len(r.items) }

func (r *rndset[T]) Add(item T) bool {
	for _, v := range r.items {
		if v == item {
			return false
		}
	}
	r.items = append(r.items, item)
	return true
}

func (r *rndset[T]) Pick() (T, bool) {
	var zero T
	if len(r.items) == 0 {
		return zero, false
	}
	// TODO: too slow, but works for now
	r.rnd.Shuffle(len(r.items), func(i, j int) { r.items[i], r.items[j] = r.items[j], r.items[i] })
	return r.items[0], true
}

func (r *rndset[T]) Remove(item T) bool {
	for i, v := range r.items {
		if v == item {
			var zero T
			all := r.items
			r.items = append(r.items[:i], r.items[i+1:]...)
			all[len(all)-1] = zero
			return true
		}
	}
	return false
}
