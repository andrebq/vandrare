package stack

import (
	"sync"
)

type (
	S[T comparable] struct {
		mutex sync.RWMutex
		items []T
	}
)

func Snapshot[T comparable, E ~[]T](i *S[T], out E) E {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	if len(i.items) == 0 {
		return out
	}
	out = append(out, i.items...)
	return out
}

func (i *S[T]) Push(v T) {
	i.mutex.Lock()
	if i.items == nil {
		i.items = make([]T, 0, 1)
	}
	i.items = append(i.items, v)
	i.mutex.Unlock()
}

func (i *S[T]) Pop() T {
	var zero T
	i.mutex.Lock()
	if len(i.items) == 0 {
		i.mutex.Unlock()
		return zero
	}

	ret := i.items[len(i.items)-1]
	i.items[len(i.items)-1] = zero
	i.items = i.items[:len(i.items)-1]
	i.mutex.Unlock()

	return ret
}

func (i *S[T]) Peek() T {
	i.mutex.Lock()
	if len(i.items) == 0 {
		var zero T
		i.mutex.Unlock()
		return zero
	}

	ret := i.items[len(i.items)-1]
	i.mutex.Unlock()

	return ret
}

func (i *S[T]) Discard(needle T) bool {
	i.mutex.Lock()
	if len(i.items) == 0 {
		i.mutex.Unlock()
		return false
	}

	found := false
	for idx, v := range i.items {
		found = v == needle
		if found {
			var zero T
			all := i.items
			i.items = append(i.items[:idx], i.items[idx+1:]...)
			// avoid a gc leak by assigning the previous top to 0
			all[len(all)-1] = zero
			break
		}
	}
	i.mutex.Unlock()
	return found
}

func (i *S[T]) Empty() bool {
	i.mutex.Lock()
	empty := len(i.items) == 0
	i.mutex.Unlock()
	return empty
}
