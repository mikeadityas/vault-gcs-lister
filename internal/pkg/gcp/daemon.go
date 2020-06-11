package gcp

import (
	"context"
	"sync"
	"time"

	"github.com/cermati/devops-toolkit/common-libs/toolkit-go/pkg/log"

	leaseUtil "github.com/mikeadityas/vault-gcs-lister/internal/pkg/util/lease"
)

const (
	maxBackoff time.Duration = 64 * time.Second
)

type Daemon interface {
	Start() error
	Stop() error
}

type daemon struct {
	gcpLeaseMgr           *GCPLeaseManager
	refreshPeriodInSecond time.Duration
	earlyRenewalInMinute  time.Duration
	ctx                   context.Context
	ctxCancelFunc         context.CancelFunc
	waitGroup             sync.WaitGroup
	ticker                *time.Ticker
	numRetry              int
	stopCh                chan bool
}

func (d *daemon) Start() error {
	d.ensureServiceAccountKey(false)

	go func() {
		for {
			select {
			case <-d.ctx.Done():
				d.stopCh <- true
				return
			case <-d.gcpLeaseMgr.forceStopCh:
				log.Logger.Sugar().Warn("GCP daemon received force stop notification")
				d.ticker.Stop()
				d.gcpLeaseMgr.NotifyAllStaleLease()
			case <-d.gcpLeaseMgr.forceNewCh:
				log.Logger.Sugar().Info("GCP daemon received force new notification")
				d.ensureServiceAccountKey(true)
			case <-d.ticker.C:
				d.ensureServiceAccountKey(false)
			}
		}
	}()
	return nil
}

// Stop stops the GCP daemon
func (d *daemon) Stop() error {
	log.Logger.Sugar().Info("Shutting down GCP Daemon...")

	d.waitGroup.Wait()
	d.ctxCancelFunc()
	<-d.stopCh
	return nil
}

func (d *daemon) ensureServiceAccountKey(isForceNew bool) {
	d.waitGroup.Add(1)
	defer d.waitGroup.Done()

	log.Logger.Sugar().Info("Ensuring GCP service account key...")
	if err := d.gcpLeaseMgr.GetNewLease(); err != nil {
		if d.numRetry == 0 {
			d.gcpLeaseMgr.NotifyAllStaleLease()
		}

		d.refreshPeriodInSecond = leaseUtil.CalculateBackoffTime(d.numRetry, maxBackoff)

		if d.ticker != nil {
			d.ticker.Stop()
		}
		d.ticker = time.NewTicker(d.refreshPeriodInSecond)

		log.Logger.Sugar().Errorw(
			"Failed to ensure GCP service account key.",
			"err", err,
			"retry.num", d.numRetry,
			"retry.interval", d.refreshPeriodInSecond,
			"retry.next", time.Now().Add(d.refreshPeriodInSecond).Format(time.RFC3339),
		)

		d.numRetry++
		return
	}

	if isForceNew {
		d.gcpLeaseMgr.NotifyAllNewLease()
	}

	d.numRetry = 0
	d.refreshPeriodInSecond = time.Duration(d.gcpLeaseMgr.ttl) * time.Second

	if d.refreshPeriodInSecond-d.earlyRenewalInMinute > 1*time.Minute {
		d.refreshPeriodInSecond = d.refreshPeriodInSecond - d.earlyRenewalInMinute
	}

	if d.ticker != nil {
		d.ticker.Stop()
	}
	d.ticker = time.NewTicker(d.refreshPeriodInSecond)

	log.Logger.Sugar().Info("GCP service acount key refreshed!")

	gcpRefreshTime := time.Now().Add(d.refreshPeriodInSecond)
	log.Logger.Sugar().Infow("Next GCP service account key refresh", "refresh_time", gcpRefreshTime.Format(time.RFC3339))
}
