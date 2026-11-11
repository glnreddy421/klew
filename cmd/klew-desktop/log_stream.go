package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/glnreddy421/klew/v2/internal/kube"
	"github.com/glnreddy421/klew/v2/internal/logstream"
	"github.com/glnreddy421/klew/v2/internal/model"
)

// MaxLogStreams caps concurrent open log tabs per kube context so per-tab
// follow budgets stay bounded.
const MaxLogStreams = 8

// LogStreamOptions picks the pods to follow: named pods, a workload or Service
// (followed through its selector, so new pods join), or a label selector.
type LogStreamOptions struct {
	// StreamID names an existing stream when MergeIntoActive is true.
	StreamID     string             `json:"streamId,omitempty"`
	Targets      []logstream.Target `json:"targets"`
	Container    string             `json:"container"`
	TailLines    int                `json:"tailLines"`
	SinceSeconds int                `json:"sinceSeconds"`
	// MaxLogRequests caps concurrent log follows for this stream tab
	// (Settings → Max concurrent log follows); 0 → 50.
	MaxLogRequests int `json:"maxLogRequests"`
	// MergeIntoActive adds these targets to the stream named by StreamID
	// instead of opening a new stream tab.
	MergeIntoActive bool `json:"mergeIntoActive"`
}

// StartLogStream starts a new log stream session and returns its status (including
// its assigned ID), or merges into StreamID when MergeIntoActive is set.
// Lines arrive on "logs:lines" (with Batch.ID) and state changes on "logs:status"
// (with Status.ID). Read-only: gets workload selectors, lists and watches pods,
// follows logs.
func (a *App) StartLogStream(opts LogStreamOptions) (logstream.Status, error) {
	client, err := a.resolveLogStreamClient()
	if err != nil {
		return logstream.Status{}, err
	}

	spec := logstream.Spec{
		Targets:      opts.Targets,
		Container:    opts.Container,
		TailLines:    opts.TailLines,
		SinceSeconds: opts.SinceSeconds,
	}
	if opts.MergeIntoActive && strings.TrimSpace(opts.StreamID) != "" {
		if cur := a.getLogStream(opts.StreamID); cur != nil {
			st := cur.Status()
			if st.Active && !st.Paused {
				if err := cur.MergeInto(a.baseContext(), spec); err != nil {
					return logstream.Status{}, err
				}
				return cur.Status(), nil
			}
		}
	}

	ctxName := a.activeContextName()
	a.logStreamMu.Lock()
	inContext := 0
	for id := range a.logStreams {
		if a.logStreamContexts[id] == ctxName {
			inContext++
		}
	}
	if inContext >= MaxLogStreams {
		a.logStreamMu.Unlock()
		return logstream.Status{}, fmt.Errorf("at most %d log streams can run at once; close a log tab first", MaxLogStreams)
	}
	a.logStreamSeq++
	id := fmt.Sprintf("log-%d", a.logStreamSeq)
	a.logStreamMu.Unlock()

	sess := &logstream.Session{
		ID:             id,
		Client:         client,
		MaxLogRequests: opts.MaxLogRequests,
		OnBatch: func(b logstream.Batch) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "logs:lines", b)
			}
		},
		// Lines from pods an investigation covers also feed its Patterns and signals.
		// A parked context's stream must not feed another cluster's investigation.
		OnLog: func(e model.EvidenceEvent) {
			if a.activeContextName() != ctxName {
				return
			}
			a.mu.Lock()
			svc := a.svc
			a.mu.Unlock()
			if svc != nil {
				svc.IngestLog(e)
			}
		},
		OnStatus: func(st logstream.Status) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "logs:status", st)
			}
		},
	}
	if err := sess.Start(a.baseContext(), spec); err != nil {
		return logstream.Status{}, err
	}
	a.logStreamMu.Lock()
	if a.logStreams == nil {
		a.logStreams = make(map[string]*logstream.Session)
	}
	a.logStreams[id] = sess
	if a.logStreamContexts == nil {
		a.logStreamContexts = make(map[string]string)
	}
	a.logStreamContexts[id] = ctxName
	a.logStreamMu.Unlock()
	return sess.Status(), nil
}

// activeContextName is the kube context new log streams are started in.
func (a *App) activeContextName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cluster.SelectedContext != "" {
		return a.cluster.SelectedContext
	}
	return a.cluster.CurrentContext
}

// LogStreamContexts maps each open log stream ID to the kube context it was
// started in, so the window can show a context's own log tabs.
func (a *App) LogStreamContexts() map[string]string {
	a.logStreamMu.Lock()
	defer a.logStreamMu.Unlock()
	out := make(map[string]string, len(a.logStreamContexts))
	for id, ctxName := range a.logStreamContexts {
		out[id] = ctxName
	}
	return out
}

// StopLogStream ends the named log stream and discards its buffered lines.
// Passing an empty id stops all running log streams.
func (a *App) StopLogStream(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		a.stopAllLogStreams()
		return
	}
	a.logStreamMu.Lock()
	sess := a.logStreams[id]
	delete(a.logStreams, id)
	delete(a.logStreamContexts, id)
	a.logStreamMu.Unlock()
	if sess != nil {
		sess.Stop()
	}
}

// PauseLogStream closes the log follows for stream id but keeps its selection and lines.
func (a *App) PauseLogStream(id string) error {
	sess := a.getLogStream(id)
	if sess == nil {
		return fmt.Errorf("no log stream %q running", id)
	}
	return sess.Pause()
}

// ResumeLogStream reopens the follows for stream id, replaying lines logged while paused.
func (a *App) ResumeLogStream(id string) error {
	sess := a.getLogStream(id)
	if sess == nil {
		return fmt.Errorf("no log stream %q running", id)
	}
	return sess.Resume(a.baseContext())
}

// LogStreamStatus reports what stream id is following.
func (a *App) LogStreamStatus(id string) logstream.Status {
	if sess := a.getLogStream(id); sess != nil {
		return sess.Status()
	}
	return logstream.Status{ID: strings.TrimSpace(id)}
}

// LogStreamLines returns the buffered lines for stream id, oldest first.
func (a *App) LogStreamLines(id string) []logstream.Line {
	if sess := a.getLogStream(id); sess != nil {
		return sess.Lines()
	}
	return nil
}

// ListLogStreams returns the statuses of all open log streams in creation order,
// so a reloaded window can restore its log tabs.
func (a *App) ListLogStreams() []logstream.Status {
	a.logStreamMu.Lock()
	sessions := make([]*logstream.Session, 0, len(a.logStreams))
	for _, sess := range a.logStreams {
		sessions = append(sessions, sess)
	}
	a.logStreamMu.Unlock()

	sort.Slice(sessions, func(i, j int) bool {
		return streamSeqNum(sessions[i].ID) < streamSeqNum(sessions[j].ID)
	})
	out := make([]logstream.Status, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, sess.Status())
	}
	return out
}

func streamSeqNum(id string) int {
	if n, err := strconv.Atoi(strings.TrimPrefix(id, "log-")); err == nil {
		return n
	}
	return 0
}

func (a *App) getLogStream(id string) *logstream.Session {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	a.logStreamMu.Lock()
	defer a.logStreamMu.Unlock()
	return a.logStreams[id]
}

func (a *App) stopAllLogStreams() {
	a.logStreamMu.Lock()
	sessions := make([]*logstream.Session, 0, len(a.logStreams))
	for _, sess := range a.logStreams {
		sessions = append(sessions, sess)
	}
	a.logStreams = nil
	a.logStreamContexts = nil
	a.logStreamMu.Unlock()
	for _, sess := range sessions {
		sess.Stop()
	}
}

func (a *App) resolveLogStreamClient() (*kube.Client, error) {
	a.logStreamMu.Lock()
	override := a.logStreamClient
	a.logStreamMu.Unlock()
	if override != nil {
		return override, nil
	}
	client, _, err := a.resolveBrowseClient("")
	return client, err
}
