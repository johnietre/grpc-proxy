package main

import "sync/atomic"

type AtomicList[T any] struct {
  head atomic.Pointer[AtomicNode[T]]
  tail atomic.Pointer[AtomicNode[T]]
  len atomic.Uint64
}

func NewAtomicList[T any]() *AtomicList[T] {
  return &AtomicList[T]{}
}

func (al *AtomicList[T]) Head() *AtomicNode[T] {
  return al.head.Load()
}

func (al *AtomicList[T]) Tail() *AtomicNode[T] {
  return al.tail.Load()
}

func (al *AtomicList[T]) Len() uint64 {
  return al.len.Load()
}

func (al *AtomicList[T]) Range(f func(*AtomicNode[T]) bool) {
  for node := al.Head(); node != nil; node = node.Next() {
    if !f(node) {
      break
    }
  }
}

func (al *AtomicList[T]) PushBack(val T) *AtomicNode[T] {
  oldLen := al.len.Add(1) - 1
  node := &AtomicNode[T]{val: val, index: oldLen}
  node.prev.Store(al.tail.Swap(node))
  if oldLen == 0 {
    al.head.Store(node)
  } else {
    for node.prev.Load() == nil {}
    node.prev.Load().next.Store(node)
  }
  return node
}

func (al *AtomicList[T]) Get(index uint64) *AtomicNode[T] {
  l := al.Len()
  if index >= l {
    return nil
  } else if index == 0 {
    return al.Head()
  } else if index == l - 1 {
    // It's possible that this is called after the length is incremented but
    // before the tail is set
    for {
      node := al.Tail()
      if node.Index() == index {
        return node
      }
    }
  } else if index < l / 2 {
    // Start from beginning
    // No need for condition since list only grows and the index is guaranteed
    // to exist.
    for node := al.Head(); node != nil; node = node.Next() {
      if node.Index() == index {
        return node
      }
    }
    // Should be unreachable
    return nil
  }
  // Start from end
  node := al.Tail()
  // Spin and wait for the tail's prev to be stored. Should be VERY short, if
  // anything.
  for i := 0; node.Prev() == nil; i++ {
    if i == 100_000 {
      panic("limit reached")
    }
  }
  // No need for condition since list only grows and the index is guaranteed
  // to exist.
  for ; node != nil; node = node.Prev() {
    if node.Index() == index {
      return node
    }
  }
  // Should be unreachable
  return nil
}

type AtomicNode[T any] struct {
  val T
  index uint64
  next atomic.Pointer[AtomicNode[T]]
  prev atomic.Pointer[AtomicNode[T]]
}

func (an *AtomicNode[T]) Value() T {
  return an.val
}

func (an *AtomicNode[T]) Index() uint64 {
  return an.index
}

func (an *AtomicNode[T]) Next() *AtomicNode[T] {
  return an.next.Load()
}

func (an *AtomicNode[T]) Prev() *AtomicNode[T] {
  return an.prev.Load()
}

func (an *AtomicNode[T]) RangeToHead(f func(*AtomicNode[T]) bool) {
  for ; an != nil; an = an.Prev() {
    if !f(an) {
      break
    }
  }
}

func (an *AtomicNode[T]) RangeToTail(f func(*AtomicNode[T]) bool) {
  for ; an != nil; an = an.Next() {
    if !f(an) {
      break
    }
  }
}
