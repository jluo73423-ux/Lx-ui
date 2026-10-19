package job

import (
	"math/rand/v2"
	"time"
)

// JitterSchedule is a cron.Schedule that fires every Interval plus a random
// extra delay in [0, Jitter). With Jitter == 0 it behaves like "@every
// Interval". Spreading ticks keeps a master's node polls from lining up with
// other periodic work and de-synchronizes fleets of masters (#6541).
type JitterSchedule struct {
	Interval time.Duration
	Jitter   time.Duration
	// rnd returns a value in [0, n); nil uses math/rand/v2. Tests override it.
	rnd func(n int64) int64
}

// NewJitterSchedule builds a schedule from second-granularity settings. The
// jitter is capped at the interval so a tick can never be delayed by more than
// one full period.
func NewJitterSchedule(intervalSec, jitterSec int) *JitterSchedule {
	if intervalSec < 1 {
		intervalSec = 1
	}
	if jitterSec < 0 {
		jitterSec = 0
	}
	if jitterSec > intervalSec {
		jitterSec = intervalSec
	}
	return &JitterSchedule{
		Interval: time.Duration(intervalSec) * time.Second,
		Jitter:   time.Duration(jitterSec) * time.Second,
	}
}

// Next implements cron.Schedule.
func (s *JitterSchedule) Next(t time.Time) time.Time {
	d := s.Interval
	if s.Jitter > 0 {
		r := s.rnd
		if r == nil {
			r = rand.Int64N
		}
		d += time.Duration(r(int64(s.Jitter)))
	}
	// Mirror cron.ConstantDelaySchedule: whole-second ticks.
	return t.Add(d - time.Duration(t.Nanosecond())*time.Nanosecond)
}
