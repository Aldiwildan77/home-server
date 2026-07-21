package worker

import (
	"context"
	"time"
)

type Worker interface {
	Run(ctx context.Context, task func(ctx context.Context) error) error
	Done() <-chan struct{}
}

type worker struct {
	ticker *time.Ticker
	done   chan struct{}
}

func NewWorker(ticker *time.Ticker) Worker {
	return &worker{
		ticker: ticker,
		done:   make(chan struct{}),
	}
}

func (w *worker) Run(ctx context.Context, task func(ctx context.Context) error) error {
	defer close(w.done)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.ticker.C:
			if err := task(ctx); err != nil {
				return err
			}
		}
	}
}

func (w *worker) Done() <-chan struct{} {
	return w.done
}
