// Package domain holds the vocabulary shared by every domain package's Run
// contract.
package domain

// Argument is one positional command-line argument as delivered to a domain
// Run. It is a type alias, not a defined type, so Run signatures written with
// it still match go-app's Runner contract (variadic string) exactly while
// naming the domain concept.
type Argument = string
