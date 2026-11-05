package keyvalue

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidURL  = errors.New("url do key value inválida")
	ErrUnreachable = errors.New("key value inalcançável")
)

func invalidURLError(cause error) error {
	return fmt.Errorf("%w: %v", ErrInvalidURL, cause)
}

func unreachableError(cause error) error {
	return fmt.Errorf("%w: %v", ErrUnreachable, cause)
}
