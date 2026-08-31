package engine

import "testing"

func TestSetLogTailPausedPreservesPods(t *testing.T) {
	st := mockBaseState()
	r := NewReducer(&st)
	r.SetLogTailPods([]string{"payment-gateway-a1", "payment-gateway-a2"})
	r.SetLogTailPaused(true)

	got := r.State()
	if !got.LogTailPaused {
		t.Fatal("expected logTailPaused")
	}
	if len(got.LogTailPods) != 2 {
		t.Fatalf("logTailPods=%v", got.LogTailPods)
	}
}

func TestPauseLogTailWithoutSession(t *testing.T) {
	var s *LiveSession
	if err := s.PauseLogTail(); err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestPauseLogTailWithoutGather(t *testing.T) {
	st := mockBaseState()
	r := NewReducer(&st)
	s := &LiveSession{Reducer: r}
	if err := s.PauseLogTail(); err == nil {
		t.Fatal("expected error when no gather session")
	}
}

func TestPauseLogTailIdempotentWhenAlreadyPaused(t *testing.T) {
	st := mockBaseState()
	r := NewReducer(&st)
	r.SetLogTailPods([]string{"payment-gateway-a1"})
	s := &LiveSession{
		Reducer:       r,
		logTailPaused: true,
		logTailNames:  []string{"payment-gateway-a1"},
	}
	if err := s.PauseLogTail(); err != nil {
		t.Fatalf("PauseLogTail: %v", err)
	}
	if !s.LogTailPaused() {
		t.Fatal("expected still paused")
	}
}

func TestResumeLogTailRequiresPaused(t *testing.T) {
	st := mockBaseState()
	r := NewReducer(&st)
	s := &LiveSession{Reducer: r}
	if err := s.ResumeLogTail(); err == nil {
		t.Fatal("expected error when not paused")
	}
}

func TestLogTailEngaged(t *testing.T) {
	st := mockBaseState()
	r := NewReducer(&st)
	s := &LiveSession{Reducer: r}

	if s.LogTailEngaged() {
		t.Fatal("expected not engaged")
	}

	r.SetLogTailPods([]string{"pod-a"})
	if !s.LogTailEngaged() {
		t.Fatal("expected engaged when pods set")
	}

	s.logTailPaused = true
	if !s.LogTailEngaged() {
		t.Fatal("expected engaged when paused")
	}
}

func TestStopLogTailClearsPausedState(t *testing.T) {
	st := mockBaseState()
	r := NewReducer(&st)
	r.SetLogTailPods([]string{"pod-a"})
	r.SetLogTailPaused(true)
	s := &LiveSession{
		Reducer:       r,
		logTailPaused: true,
		logTailNames:  []string{"pod-a"},
	}
	s.StopLogTail()
	got := r.State()
	if got.LogTailPaused || len(got.LogTailPods) > 0 {
		t.Fatalf("expected cleared state, got paused=%v pods=%v", got.LogTailPaused, got.LogTailPods)
	}
	if s.logTailPaused {
		t.Fatal("session still marked paused")
	}
}

func TestReducerCloneCopiesLogTailPods(t *testing.T) {
	st := mockBaseState()
	st.LogTailPods = []string{"a", "b"}
	st.LogTailPaused = true
	got := st.Clone()
	if len(got.LogTailPods) != 2 || !got.LogTailPaused {
		t.Fatalf("clone lost tail state: %#v", got)
	}
	got.LogTailPods[0] = "mutated"
	if st.LogTailPods[0] == "mutated" {
		t.Fatal("clone should be deep copy for logTailPods")
	}
}
