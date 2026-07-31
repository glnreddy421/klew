package model

// StreamViewMode controls the bottom live evidence panel.
type StreamViewMode int

const (
	StreamRanked StreamViewMode = iota
	StreamRawLogs
	StreamK8sEvents
	StreamErrorsOnly
)

func (m StreamViewMode) String() string {
	switch m {
	case StreamRawLogs:
		return "raw logs"
	case StreamK8sEvents:
		return "k8s events"
	case StreamErrorsOnly:
		return "errors only"
	default:
		return "ranked evidence"
	}
}

func (m StreamViewMode) Next() StreamViewMode {
	return StreamViewMode((int(m) + 1) % 4)
}
