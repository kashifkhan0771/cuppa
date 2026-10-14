package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/kashifkhan0771/cuppa/internal/scheduler"
)

const (
	// daemonTick is the longest the daemon sleeps in one go. Waking up
	// every minute means a laptop coming back from sleep, or a clock
	// change, is noticed quickly.
	daemonTick = time.Minute

	// missedAfter: a post this late (the computer was asleep or off at
	// post time) is skipped, so a morning question doesn't go out at night.
	missedAfter = time.Hour
)

// runDaemon keeps running and posts whenever schedule.cron says so.
// It stops cleanly on Ctrl+C or SIGTERM. A failed post is logged and
// the daemon carries on with the next one.
func runDaemon(a *app, args []string) error {
	fs := a.newFlagSet("daemon", "daemon [--dry-run]")
	dryRun := fs.Bool("dry-run", false, "log what would be posted, without posting")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if err := a.loadConfig(); err != nil {
		return err
	}
	if a.cfg.Path == "" {
		return configError(errNoConfig)
	}

	// build everything first, so a config mistake stops the daemon
	// right away instead of at the first post time
	schedule, err := a.buildSchedule()
	if err != nil {
		return err
	}
	if schedule == nil {
		return configError(errors.New("schedule.cron is empty; the daemon needs it to know when to post"))
	}
	p, err := a.newPoster("", a.cfg.Selection)
	if err != nil {
		return err
	}

	for {
		due, err := schedule.Next(a.now())
		if err != nil {
			return configError(err)
		}
		a.log.Info("next post", "at", due.Format(time.RFC3339))
		if err := a.waitUntil(due); err != nil {
			a.log.Info("stopped")
			return nil
		}

		if late := a.now().Sub(due); late > missedAfter {
			a.log.Warn("missed a post time, skipping it", "due", due.Format(time.RFC3339), "late", late.Round(time.Minute).String())
			continue
		}
		a.postScheduled(p, *dryRun)
	}
}

// waitUntil sleeps until the given time, in steps of at most
// daemonTick. It returns an error when cuppa is being stopped.
func (a *app) waitUntil(due time.Time) error {
	for {
		left := due.Sub(a.now())
		if left <= 0 {
			return nil
		}
		if err := a.sleep(a.ctx, min(left, daemonTick)); err != nil {
			return err
		}
	}
}

// postScheduled makes one scheduled post. Problems are logged, not
// returned, so one bad post never stops the daemon.
func (a *app) postScheduled(p *poster, dryRun bool) {
	out, err := a.prepare(p)
	if err != nil {
		a.log.Error("could not pick a question", "error", err)
		return
	}
	if dryRun {
		for i, t := range p.targets {
			a.log.Info("would post", "target", t.name, "message", out.messages[i])
		}
		return
	}
	if _, err := a.send(p, out); err != nil {
		a.log.Error("post did not fully work", "error", err)
	}
}

// buildSchedule parses the schedule part of the config. It returns
// nil when schedule.cron is not set.
func (a *app) buildSchedule() (*scheduler.Schedule, error) {
	s := a.cfg.Schedule
	if s.Cron == "" {
		return nil, nil
	}
	schedule, err := scheduler.New(scheduler.Options{
		Cron:         s.Cron,
		Location:     a.cfg.Location,
		SkipWeekends: s.SkipWeekends,
		SkipDates:    s.SkipDates,
	})
	if err != nil {
		return nil, configError(fmt.Errorf("schedule: %w", err))
	}
	return schedule, nil
}
