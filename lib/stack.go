package lib

import (
	"sync"

	"github.com/huandu/go-clone"
)

type Stack[T any] struct {
	lock sync.Mutex // you don't have to do this if you don't want thread safety
	s    []T
}

func NewStack[T any]() *Stack[T] {
	return &Stack[T]{sync.Mutex{}, make([]T, 0)}
}

func (s *Stack[T]) Push(v T) *Stack[T] {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.s = append(s.s, v)
	return s
}

func (s *Stack[T]) Pop() (T, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	var v T
	l := len(s.s)
	if l == 0 {
		return v, false
	}

	res := s.s[l-1]
	s.s = s.s[:l-1]
	return res, true
}

func (s *Stack[T]) Peek() (T, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	l := len(s.s)
	if l == 0 {
		var v T
		return v, false
	}

	res := s.s[l-1]
	return res, true
}

func (s *Stack[T]) Clone() *Stack[T] {
	s.lock.Lock()
	defer s.lock.Unlock()
	other := NewStack[T]()
	clonedArr := clone.Clone(s.s)
	for _, v := range clonedArr.([]T) {
		other.Push(v)
	}
	return other
}

func (s *Stack[T]) Len() int {
	s.lock.Lock()
	defer s.lock.Unlock()
	return len(s.s)
}
