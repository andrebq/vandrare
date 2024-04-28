package store

import "errors"

var (
	errNotFound = errors.New("not found")
)

func IsNotFound(err error) bool {
	return errors.Is(err, errNotFound)
}
