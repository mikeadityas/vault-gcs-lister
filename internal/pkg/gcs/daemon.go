package gcs

import (
	"context"
	"strings"
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
	bucketListerSvc              *BucketListerService
	currentRefreshPeriodInSecond time.Duration
	desiredRefreshPeriodInSecond time.Duration
	ctx                          context.Context
	ctxCancelFunc                context.CancelFunc
	waitGroup                    sync.WaitGroup
	ticker                       *time.Ticker
	numRetry                     int
	stopCh                       chan bool
}

func (d *daemon) Start() error {
	d.listBucket()
	go func() {
		for {
			select {
			case <-d.ctx.Done():
				d.stopCh <- true
				return
			case <-d.bucketListerSvc.forceStopCh:
				log.Logger.Sugar().Warn("GCS daemon received force stop notification")
				d.ticker.Stop()
			case <-d.bucketListerSvc.forceNewCh:
				log.Logger.Sugar().Info("GCS daemon received force new notification")
				d.listBucket()
			case <-d.ticker.C:
				d.listBucket()
			}
		}
	}()
	return nil
}

// Stop stops the GCP daemon
func (d *daemon) Stop() error {
	log.Logger.Sugar().Info("Shutting down GCS Daemon...")

	d.waitGroup.Wait()
	d.ctxCancelFunc()
	<-d.stopCh
	return nil
}

func (d *daemon) listBucket() {
	d.waitGroup.Add(1)
	defer d.waitGroup.Done()

	log.Logger.Sugar().Infow("Listing GCS buckets", "project_id", d.bucketListerSvc.projectID)
	buckets, err := d.bucketListerSvc.ListBucket()
	if err != nil {
		d.currentRefreshPeriodInSecond = leaseUtil.CalculateBackoffTime(d.numRetry, maxBackoff)

		if d.ticker != nil {
			d.ticker.Stop()
		}
		d.ticker = time.NewTicker(d.currentRefreshPeriodInSecond)

		log.Logger.Sugar().Errorw(
			"Failed to list GCS buckets.",
			"project_id", d.bucketListerSvc.projectID,
			"err", err,
			"retry.num", d.numRetry,
			"retry.interval", d.currentRefreshPeriodInSecond,
			"retry.next", time.Now().Add(d.currentRefreshPeriodInSecond).Format(time.RFC3339),
		)

		d.numRetry++
		return
	}

	log.Logger.Sugar().Infof("Buckets in %s: %s", d.bucketListerSvc.projectID, strings.Join(buckets, ", "))
	d.numRetry = 0
	d.currentRefreshPeriodInSecond = time.Duration(d.desiredRefreshPeriodInSecond)

	if d.ticker != nil {
		d.ticker.Stop()
	}
	d.ticker = time.NewTicker(d.currentRefreshPeriodInSecond)

	gcsNextListing := time.Now().Add(d.currentRefreshPeriodInSecond)
	log.Logger.Sugar().Infow("Next GCS listing", "listing_time", gcsNextListing.Format(time.RFC3339))
}
