package main

import (
  "math"
	"math/rand"
)

type UpDown interface {
  Up(float64) float64
  Down(float64) float64
}

type AbsUpDown struct {
  MaxUp, MaxDown float64
}

func NewAbsUpDown(up, down float64) AbsUpDown {
  return AbsUpDown{up, down}
}

func (aud AbsUpDown) Up(f float64) float64 {
  return f + rand.Float64() * aud.MaxUp
}

func (aud AbsUpDown) Down(f float64) float64 {
  return f - rand.Float64() * aud.MaxDown
}

type PctUpDown struct {
  MaxUp, MaxDown float64
}

func NewPctUpDown(up, down float64) PctUpDown {
  return PctUpDown{up, down}
}

func (aud PctUpDown) Up(f float64) float64 {
  return f * 1 + rand.Float64() * aud.MaxUp
}

func (aud PctUpDown) Down(f float64) float64 {
  return f * 1 + rand.Float64() * aud.MaxDown
}

type Walker interface {
  Next() float64
  Value() float64
}

type RandWalker struct {
  val float64
  upDown UpDown
  upWeight float64
}

// upWeight is the ratio of price increases to decreases over a number of
// periods.
func NewRandWalker(
  initial float64,
  upDown UpDown,
  upWeight float64,
) *RandWalker {
  return &RandWalker{
    val: initial,
    upDown: upDown,
    upWeight: upWeight,
  }
}

func (rw *RandWalker) Next() float64 {
  up := rand.Float64() < rw.upWeight
  if up {
    rw.val = rw.upDown.Up(rw.val)
  } else {
    rw.val = rw.upDown.Down(rw.val)
  }
  return rw.val
}

func (rw *RandWalker) Value() float64 {
  return rw.val
}

type RandPctWalker struct {
  // The current value
  val float64
  // The amounts to increase/decrease the value by (multiplication).
  // E.g., if you want to increase/decrease by 10%/5%, incr will be 1.1 and
  // decr will be 0.95
  incr, decr float64
  // The threshold for a random number [0.0, 1.0) to signify an up or down
  // movement. If the number is less than mThres, go up, otherwise, go down.
  mThres float64
}

// up and down are the percent increase and decrease, respectively.
// growth is the desired percent change from the initial value by the end of
// the period. E.g., if initial is 100 and desired is 90, growth is -0.1.
func NewRandPctWalker(
  initial, up, down, growth float64,
  period uint64,
) *RandPctWalker {
  U, D, R, N := 1.0 + up, 1.0 - down, growth + 1.0, float64(period)
  lnR := math.Log(R)
  m := (N * math.Log(D) - lnR) / math.Log(U / D)
  mThres := m / N
  return &RandPctWalker{
    val: initial,
    incr: U,
    decr: D,
    mThres: mThres,
  }
}

func (rpw *RandPctWalker) Next() float64 {
  up := rand.Float64() < rpw.mThres
  if up {
    rpw.val *= rpw.incr
  } else {
    rpw.val *= rpw.decr
  }
  return rpw.val
}

func (rpw *RandPctWalker) Value() float64 {
  return rpw.val
}
