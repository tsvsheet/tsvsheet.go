package config

import (
	store "github.com/gomatic/template.cli/internal/config"
	"github.com/gomatic/template.cli/internal/constants"
	"github.com/gomatic/template.cli/internal/domain"
)

// KeyFrom extracts and validates the configuration key from the positional args.
func KeyFrom(args ...domain.Argument) (store.Key, error) {
	if len(args) < 1 {
		return "", constants.ErrMissingArgument.With(nil, "key")
	}
	if args[0] == "" {
		return "", constants.ErrInvalidName.With(nil, "key cannot be empty")
	}
	return store.Key(args[0]), nil
}

// PairFrom extracts and validates the key and value from the positional args.
func PairFrom(args ...domain.Argument) (store.Key, store.Value, error) {
	if len(args) < 1 {
		return "", "", constants.ErrMissingArgument.With(nil, "key and value")
	}
	if len(args) < 2 {
		return "", "", constants.ErrMissingArgument.With(nil, "value")
	}
	key, err := KeyFrom(args...)
	if err != nil {
		return "", "", err
	}
	if args[1] == "" {
		return "", "", constants.ErrInvalidValue.With(nil, "value cannot be empty")
	}
	return key, store.Value(args[1]), nil
}
