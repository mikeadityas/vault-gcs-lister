package vault

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

// Daemon is the interface for Vault daemon which renews the Vault token
type Daemon interface {
	Start() error
	Stop() error
}

type daemon struct {
	vaultLeaseMgr         *VaultLeaseManager
	refreshPeriodInSecond time.Duration
	ctx                   context.Context
	ctxCancelFunc         context.CancelFunc
	waitGroup             sync.WaitGroup
	ticker                *time.Ticker
	numRetry              int
	stopCh                chan bool
}

// Start starts the Vault daemon
func (d *daemon) Start() error {
	go func() {
		for {
			select {
			case <-d.ctx.Done():
				d.stopCh <- true
				return
			case <-d.ticker.C:
				d.vaultLeaseMgr.NotifyAllStaleLease()
				d.ensureToken()
			}
		}
	}()
	return nil
}

// Stop stops the Vault daemon
func (d *daemon) Stop() error {
	log.Logger.Sugar().Info("Shutting down Vault Daemon...")

	d.waitGroup.Wait()
	d.ctxCancelFunc()
	<-d.stopCh
	return nil
}

func (d *daemon) ensureToken() {
	d.waitGroup.Add(1)
	defer d.waitGroup.Done()

	log.Logger.Sugar().Info("Ensuring Vault token...")
	if err := d.vaultLeaseMgr.client.EnsureToken(); err != nil {
		if d.numRetry == 0 {
			d.vaultLeaseMgr.NotifyAllStaleLease()
		}

		d.refreshPeriodInSecond = leaseUtil.CalculateBackoffTime(d.numRetry, maxBackoff)

		if d.ticker != nil {
			d.ticker.Stop()
		}

		d.ticker = time.NewTicker(d.refreshPeriodInSecond)

		log.Logger.Sugar().Errorw(
			"Failed to ensure Vault token.",
			"err", err,
			"retry.num", d.numRetry,
			"retry.interval", d.refreshPeriodInSecond,
			"retry.next", time.Now().Add(d.refreshPeriodInSecond).Format(time.RFC3339),
		)

		d.numRetry++
		return
	}

	d.vaultLeaseMgr.NotifyAllNewLease()

	d.numRetry = 0
	d.refreshPeriodInSecond = time.Duration(d.vaultLeaseMgr.client.TTL()) * time.Second

	if d.ticker != nil {
		d.ticker.Stop()
	}

	d.ticker = time.NewTicker(d.refreshPeriodInSecond)

	log.Logger.Sugar().Info("Vault token renewed!")

	vaultRenewTime := time.Now().Add(d.refreshPeriodInSecond)
	log.Logger.Sugar().Infow("Next Vault token renew", "renew_time", vaultRenewTime.Format(time.RFC3339))
}
