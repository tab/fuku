package services

import (
	"sync"

	"fuku/internal/contracts"
)

// stopAll cancels the active run so Run stops every service (the control that published the command closed admission)
func (r *Runtime) stopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancel == nil {
		return
	}

	r.log.Info("Received StopAll command, shutting down all services...")
	r.cancel(errStopAll)
}

// end cancels and releases the run under the launch lock, so no action is dispatched and no child starts once it ended
func (r *Runtime) end() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cancel(nil)
	r.work = nil
	r.cancel = nil
}

// shutdown ends the run, joins the in-flight actions and stops the tracked children newest first, a tier at a time
func (r *Runtime) shutdown() int {
	r.end()
	r.wg.Wait()

	procs := r.tracker.Reverse()

	for _, proc := range procs {
		r.tracker.Detach(proc.Service().ID)
	}

	for _, tier := range tierRuns(procs) {
		var stopping sync.WaitGroup

		for _, proc := range tier {
			stopping.Go(func() { r.stop(proc.Service().ID) })
		}

		stopping.Wait()
	}

	return len(procs)
}

// tierRuns splits children listed newest first into runs of one tier, keeping their order
func tierRuns(procs []contracts.Process) [][]contracts.Process {
	var runs [][]contracts.Process

	for _, proc := range procs {
		last := len(runs) - 1
		sameTier := last >= 0 && runs[last][0].Service().Tier == proc.Service().Tier

		if sameTier {
			runs[last] = append(runs[last], proc)

			continue
		}

		runs = append(runs, []contracts.Process{proc})
	}

	return runs
}
