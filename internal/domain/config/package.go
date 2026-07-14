// Package config holds the domain logic shared by the config verbs (get, set,
// list): extracting and validating configuration keys and values from the
// positional arguments. The verbs live in the subpackages get, list, and set;
// the store they operate on is the implementation tier (internal/config),
// imported here as store.
package config
