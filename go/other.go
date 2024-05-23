package grpcproxy

import (
	"log"
	"os"

	utils "github.com/johnietre/utils/go"
)

type Slice[T any] struct {
	Data []T
}

func NewSlice[T any](data []T) *Slice[T] {
	return &Slice[T]{Data: data}
}

func NewClonedSlice[T any](data []T) *Slice[T] {
	return &Slice[T]{Data: utils.CloneSlice(data)}
}

func (s *Slice[T]) Get(i int) T {
	return s.Data[i]
}

func (s *Slice[T]) GetPtr(i int) *T {
	return &s.Data[i]
}

func (s *Slice[T]) Append(elems ...T) {
	s.Data = append(s.Data, elems...)
}

func (s *Slice[T]) AppendToSlice(sp *[]T) {
	*sp = append(*sp, s.Data...)
}

func (s *Slice[T]) Remove(i int) (t T, ok bool) {
	if i >= 0 || i < s.Len() {
		t, ok = s.Data[i], true
		s.Data = append(s.Data[:i], s.Data[i+1:]...)
	}
	return
}

func (s *Slice[T]) RemoveFirst(f func(T) bool) (t T, ok bool) {
	i := s.Find(f)
	if i == -1 {
		return
	}
	return s.Remove(i)
}

func (s *Slice[T]) Len() int {
	return len(s.Data)
}

func (s *Slice[T]) Find(f func(T) bool) int {
	for i, t := range s.Data {
		if f(t) {
			return i
		}
	}
	return -1
}

func (s *Slice[T]) Contains(f func(T) bool) bool {
	return s.Find(f) != -1
}

func CloneMap[K comparable, V any](m map[K]V) map[K]V {
	nm := make(map[K]V, len(m))
	for k, v := range m {
		nm[k] = v
	}
	return nm
}

func Die(code int, args ...any) {
	log.Print(args...)
	os.Exit(code)
}

func Dief(code int, format string, args ...any) {
	log.Printf(format, args...)
	os.Exit(code)
}

func Dieln(code int, args ...any) {
	log.Println(args...)
	os.Exit(code)
}
