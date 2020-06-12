package vault

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/cermati/devops-toolkit/common-libs/toolkit-go/pkg/csvault"
	"github.com/cermati/devops-toolkit/common-libs/toolkit-go/pkg/cvault"
	"github.com/mikeadityas/vault-gcs-lister/internal/pkg/config"
	leaseMgr "github.com/mikeadityas/vault-gcs-lister/internal/pkg/leasemanager"
)

var instance *VaultLeaseManager

type VaultLeaseManager struct {
	childLeases []leaseMgr.Observer

	client cvault.CVault
}

func (vlm *VaultLeaseManager) Register(childLease leaseMgr.Observer) {
	vlm.childLeases = append(vlm.childLeases, childLease)
}

func (vlm *VaultLeaseManager) Deregister(childLease leaseMgr.Observer) {
	vlm.childLeases = leaseMgr.RemoveObserver(vlm.childLeases, childLease)
}

func (vlm *VaultLeaseManager) NotifyAllNewLease() {
	for _, childLease := range vlm.childLeases {
		childLease.NotifyNewLease()
	}
}

func (vlm *VaultLeaseManager) NotifyAllStaleLease() {
	for _, childLease := range vlm.childLeases {
		childLease.NotifyStaleLease()
	}
}

func (vlm *VaultLeaseManager) Client() cvault.CVault {
	return vlm.client
}

func (vlm *VaultLeaseManager) Daemonize(
	ctx context.Context,
	ctxCancelFunc context.CancelFunc,
	refreshPeriodInSecond time.Duration,
) Daemon {
	tick := time.NewTicker(refreshPeriodInSecond)

	return &daemon{
		vaultLeaseMgr:         vlm,
		refreshPeriodInSecond: refreshPeriodInSecond,
		ctx:                   ctx,
		ctxCancelFunc:         ctxCancelFunc,
		waitGroup:             sync.WaitGroup{},
		ticker:                tick,
		numRetry:              0,
		stopCh:                make(chan bool),
	}
}

func NewVaultLeaseManager(vaultConf *config.VaultConfig, tlsConf *config.TLSConfig) error {
	vaultConfig, err := cvault.GenVaultConfig(
		vaultConf.Address,
		true,
		tlsConf.CACertPath,
		tlsConf.CertPath,
		tlsConf.KeyPath,
	)
	if err != nil {
		return errors.Wrap(err, "failed to generate Vault config")
	}

	client, err := csvault.NewCSVault(vaultConfig, vaultConf.RoleName)
	if err != nil {
		return errors.Wrap(err, "failed to initialize Vault client")
	}

	if err := client.EnsureToken(); err != nil {
		return errors.Wrap(err, "failed to ensure Vault token")
	}

	instance = &VaultLeaseManager{
		client: client,
	}

	return nil
}

func GetInstance() *VaultLeaseManager {
	if instance == nil {
		panic("The Vault lease manager instance hasn't been initialized")
	}
	return instance
}
