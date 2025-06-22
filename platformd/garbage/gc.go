/*
 Explorer Platform, a platform for hosting and discovering Minecraft servers.
 Copyright (C) 2024 Yannic Rieger <oss@76k.io>

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU Affero General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 GNU Affero General Public License for more details.

 You should have received a copy of the GNU Affero General Public License
 along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package garbage

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Collector interface {
	CollectGarbage(ctx context.Context) error
}

type Executor struct {
	logger     *slog.Logger
	collectors []Collector
	ticker     *time.Ticker
	stop       chan bool
}

func NewExecutor(logger *slog.Logger, interval time.Duration, collectors ...Collector) *Executor {
	return &Executor{
		logger:     logger,
		collectors: collectors,
		ticker:     time.NewTicker(interval),
		stop:       make(chan bool),
	}
}

func (e *Executor) Run(ctx context.Context) {
	for {
		select {
		case <-e.ticker.C:
			var wg sync.WaitGroup
			for _, c := range e.collectors {
				wg.Add(1)
				go func() {
					if err := c.CollectGarbage(ctx); err != nil {
						e.logger.ErrorContext(ctx, "collector run failed", "err", err)
					}
					wg.Done()
				}()
			}
			wg.Wait()
		case <-e.stop:
			return
		}
	}
}

func (e *Executor) Stop() {
	e.ticker.Stop()
	e.stop <- true
}
