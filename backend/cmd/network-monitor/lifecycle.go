package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

func (a *app) run(ctx context.Context) error {
	if a == nil || a.srv == nil {
		return errors.New("application not initialized")
	}
	defer a.pools.Close()
	defer a.cancel()

	go func() {
		slog.Info("backend listening", "addr", a.listenAddr)
		if err := a.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			a.cancel()
		}
	}()

	select {
	case <-ctx.Done():
	case <-a.ctx.Done():
	}
	slog.Info("shutdown signal received")

	// Parent ctx уже Done — бюджеты shutdown наследуют values, но не cancel.
	base := context.WithoutCancel(ctx)
	httpCtx, httpCancel := context.WithTimeout(base, 15*time.Second)
	if err := a.srv.Shutdown(httpCtx); err != nil {
		slog.Warn("http shutdown failed", "err", err)
	} else {
		slog.Info("http shutdown complete")
	}
	httpCancel()

	geoCtx, geoCancel := context.WithTimeout(base, 5*time.Second)
	a.geoJobs.Shutdown(geoCtx)
	geoCancel()

	repCtx, repCancel := context.WithTimeout(base, 5*time.Second)
	a.repJobs.Shutdown(repCtx)
	repCancel()

	ingestWait := a.ingestSvc.ShutdownWaitTimeout()
	snap := a.ingestSvc.Stats()
	slog.Info("waiting for ingest drain",
		"budget", ingestWait.String(),
		"queue_depth", snap.QueueDepth,
		"dropped_total", snap.DroppedTotal,
	)
	waitCtx, waitCancel := context.WithTimeout(base, ingestWait)
	select {
	case err := <-a.ingestDone:
		if err != nil {
			slog.Warn("ingest shutdown error", "err", err)
		} else {
			slog.Info("ingest shutdown complete")
		}
	case <-waitCtx.Done():
		slog.Warn("ingest drain timeout",
			"queue_depth_left", a.ingestSvc.Stats().QueueDepth,
			"budget", ingestWait.String(),
		)
		a.ingestSvc.AbortDrain()
		select {
		case <-a.ingestDone:
		case <-time.After(3 * time.Second):
			slog.Warn("ingest workers still draining after AbortDrain")
		}
	}
	waitCancel()

	a.bgCancel()
	bgWaitCtx, bgWaitCancel := context.WithTimeout(base, 5*time.Second)
	defer bgWaitCancel()
	done := make(chan struct{})
	go func() {
		a.bgWg.Wait()
		close(done)
	}()
	select {
	case <-done:
		slog.Info("background workers stopped")
	case <-bgWaitCtx.Done():
		slog.Warn("background workers drain timeout")
	}

	slog.Info("shutdown complete")
	return nil
}
