package server

import "github.com/lyhbdw/komari-lite/agent/ws"

// No backlog: capacity is acquired before creating a worker and held through
// result upload, so slow probes and slow controllers both apply backpressure.
const maxActivePingTasks = 8

type pingJob struct {
	conn         *ws.SafeConn
	taskID       uint
	kind, target string
}
type pingGate struct {
	slots chan struct{}
	run   func(pingJob)
}

func newPingAdmission(limit int, run func(pingJob)) *pingGate {
	return &pingGate{slots: make(chan struct{}, limit), run: run}
}

var pingAdmission = newPingAdmission(maxActivePingTasks, func(job pingJob) {
	NewPingTask(job.conn, job.taskID, job.kind, job.target)
})

func (g *pingGate) submit(job pingJob) bool {
	select {
	case g.slots <- struct{}{}:
		go func() { defer func() { <-g.slots }(); g.run(job) }()
		return true
	default:
		return false
	}
}
