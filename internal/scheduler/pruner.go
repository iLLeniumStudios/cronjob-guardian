/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scheduler

import (
	"context"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// HistoryPruner periodically removes old execution records
type HistoryPruner struct {
	store         store.Store
	retentionDays int
	interval      time.Duration
	stopCh        chan struct{}
	running       bool
	mu            sync.Mutex
}

// NewHistoryPruner creates a new history pruner
func NewHistoryPruner(st store.Store, retentionDays int) *HistoryPruner {
	return &HistoryPruner{
		store:         st,
		retentionDays: retentionDays,
		interval:      6 * time.Hour,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the pruner loop
func (p *HistoryPruner) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return nil
	}
	p.running = true
	p.mu.Unlock()

	logger := log.FromContext(ctx)
	logger.Info("starting history pruner", "retentionDays", p.retentionDays, "interval", p.interval)

	// Run immediately on start
	p.prune(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.stopCh:
			return nil
		case <-ticker.C:
			p.prune(ctx)
		}
	}
}

// Stop halts the pruner
func (p *HistoryPruner) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		close(p.stopCh)
		p.running = false
	}
}

// SetRetentionDays changes the retention period
func (p *HistoryPruner) SetRetentionDays(days int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.retentionDays = days
}

// SetInterval changes the prune interval
func (p *HistoryPruner) SetInterval(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.interval = d
}

func (p *HistoryPruner) prune(ctx context.Context) {
	logger := log.FromContext(ctx)

	p.mu.Lock()
	retentionDays := p.retentionDays
	p.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	count, err := p.store.Prune(ctx, cutoff)
	if err != nil {
		logger.Error(err, "failed to prune history")
		return
	}

	if count > 0 {
		logger.Info("pruned execution history", "recordsDeleted", count, "cutoff", cutoff)
	}
}
