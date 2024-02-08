package lib

import (
	"github.com/huandu/go-clone"
	"sync"
)

type Stack[T any] struct {
	lock sync.Mutex // you don't have to do this if you don't want thread safety
	s    []T
	// peak is a pointer to the last element in the stack
	// When doing profiling, peak makes it easier to get the last element in the stack
	// and it is used a huge number of times
	peak *T
}

func NewStack[T any]() *Stack[T] {
	return &Stack[T]{sync.Mutex{}, make([]T, 0, 10), nil}
}

func (s *Stack[T]) Push(v T) *Stack[T] {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.s = append(s.s, v)
	s.peak = &v
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

	s.peak = nil
	res := s.s[l-1]
	s.s = s.s[:l-1]
	if l > 1 {
		s.peak = &s.s[l-2]
	}
	return res, true
}

func (s *Stack[T]) Peek() (T, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.peak != nil {
		return *s.peak, true
	} else {
		var v T
		return v, false
	}
	//l := len(s.s)
	//if l == 0 {
	//	var v T
	//	return v, false
	//}
	//
	//res := s.s[l-1]
	//s.peak = &res
	//return res, true
}

func (s *Stack[T]) Clone() *Stack[T] {
	s.lock.Lock()
	defer s.lock.Unlock()
	other := NewStack[T]()
	clonedArr := s.RawArrayCopy()
	for _, v := range clonedArr {
		other.Push(v)
	}
	return other
}

func (s *Stack[T]) RawArrayCopy() []T {
	return clone.Clone(s.s).([]T)
}

func (s *Stack[T]) Len() int {
	s.lock.Lock()
	defer s.lock.Unlock()
	return len(s.s)
}
