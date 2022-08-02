package admin

import "reflect"

// Pair Pair
type Pair[T any, S any] struct {
	First  T
	Second S
}

// Model is the wrapper containing ORM and configs.
type Model struct {
	ORM      interface{}
	Filters  []Pair[string, reflect.Type]
	Preloads []string
}
