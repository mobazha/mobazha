package core

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/mobazha/mobazha/internal/logger"
	pkgscheduler "github.com/mobazha/mobazha/pkg/scheduler"
)

// startStandaloneSchedulerPlan owns the distribution-neutral scheduler
// lifecycle. A nil jobNames slice registers every available hook; a concrete
// slice is an explicit product policy and is validated against the hook set.
func (n *MobazhaNode) startStandaloneSchedulerPlan(
	ctx context.Context,
	jobNames []string,
	hookFns map[string]func(context.Context) error,
) {
	if len(hookFns) == 0 {
		return
	}

	hostname, _ := os.Hostname()
	holderID := fmt.Sprintf("standalone-%s-%s", hostname, n.nodeID)
	sched, err := pkgscheduler.New(pkgscheduler.Config{HolderID: holderID})
	if err != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to create standalone scheduler: %v", err)
		return
	}

	if jobNames == nil {
		jobNames = make([]string, 0, len(hookFns))
		for name := range hookFns {
			jobNames = append(jobNames, name)
		}
		sort.Strings(jobNames)
	}

	for _, name := range jobNames {
		fn, available := hookFns[name]
		if !available {
			logger.LogErrorWithIDf(log, n.nodeID, "Standalone policy selected unavailable job %q", name)
			continue
		}
		meta, ok := pkgscheduler.Jobs[name]
		if !ok {
			logger.LogErrorWithIDf(log, n.nodeID, "Standalone scheduler: unknown job %q not in pkg/scheduler.Jobs", name)
			continue
		}
		if regErr := sched.Register(pkgscheduler.Job{
			Name:          meta.Name,
			Interval:      meta.Interval,
			GlobalFn:      fn,
			OverlapPolicy: meta.OverlapPolicy,
		}); regErr != nil {
			logger.LogErrorWithIDf(log, n.nodeID, "Failed to register standalone job %q: %v", name, regErr)
		}
	}

	if startErr := sched.Start(ctx); startErr != nil {
		logger.LogErrorWithIDf(log, n.nodeID, "Failed to start standalone scheduler: %v", startErr)
		return
	}

	go func() {
		<-ctx.Done()
		sched.Stop()
	}()
}
