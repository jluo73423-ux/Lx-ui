package job

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestJitterScheduleNoJitterMatchesEvery(t *testing.T) {
	s := NewJitterSchedule(5, 0)
	every := cron.Every(5 * time.Second)
	base := time.Date(2026, 10, 19, 12, 0, 0, 123456789, time.UTC)
	if got, want := s.Next(base), every.Next(base); !got.Equal(want) {
		t.Fatalf("Next = %v, want %v", got, want)
	}
}

func TestJitterScheduleAddsBoundedDelay(t *testing.T) {
	s := NewJitterSchedule(10, 3)
	base := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	var calledWith int64
	s.rnd = func(n int64) int64 { calledWith = n; return n - 1 }
	got := s.Next(base)
	if calledWith != int64(3*time.Second) {
		t.Fatalf("rnd bound = %d, want %d", calledWith, int64(3*time.Second))
	}
	if d := got.Sub(base); d < 10*time.Second || d >= 13*time.Second {
		t.Fatalf("delay %v outside [10s, 13s)", d)
	}
	for range 200 {
		s.rnd = nil
		d := s.Next(base).Sub(base)
		if d < 10*time.Second || d >= 13*time.Second {
			t.Fatalf("random delay %v outside [10s, 13s)", d)
		}
	}
}

func TestNewJitterScheduleClamps(t *testing.T) {
	s := NewJitterSchedule(0, -1)
	if s.Interval != time.Second || s.Jitter != 0 {
		t.Fatalf("got %v/%v, want 1s/0", s.Interval, s.Jitter)
	}
	s = NewJitterSchedule(5, 60)
	if s.Jitter != 5*time.Second {
		t.Fatalf("jitter %v not capped at interval", s.Jitter)
	}
}
