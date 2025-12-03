package task

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Task struct {
	Name     string
	Interval time.Duration
	Execute  func() error
	access   sync.Mutex
	running  bool
	stop     chan struct{}
}

func (t *Task) Start(first bool) error {
	t.access.Lock()
	if t.running {
		t.access.Unlock()
		return nil
	}
	t.running = true
	t.stop = make(chan struct{})
	t.access.Unlock()

	go func() {
		timer := time.NewTimer(t.Interval)
		defer timer.Stop()
		if first {
			if err := t.Execute(); err != nil {
				t.safeStop()
				return
			}
		}

		for {
			timer.Reset(t.Interval)
			select {
			case <-timer.C:
				// continue
			case <-t.stop:
				return
			}

			if err := t.Execute(); err != nil {
				log.Errorf("Task %s execution error: %v", t.Name, err)
				t.safeStop()
				return
			}
		}
	}()

	return nil
}

func (t *Task) safeStop() {
	t.access.Lock()
	if t.running {
		t.running = false
		close(t.stop)
	}
	t.access.Unlock()
}

func (t *Task) Close() {
	t.safeStop()
	log.Warningf("Task %s stopped", t.Name)
}
