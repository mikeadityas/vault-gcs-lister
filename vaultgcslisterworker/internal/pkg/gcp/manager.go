package gcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/cermati/devops-toolkit/common-libs/toolkit-go/pkg/cvault"
	"github.com/cermati/devops-toolkit/common-libs/toolkit-go/pkg/log"
	leaseMgr "github.com/mikeadityas/vault-gcs-lister/internal/pkg/leasemanager"
)

type GCPLeaseManager struct {
	id                string
	client            cvault.CVault
	secretsPath       string
	serviceAccountKey []byte
	ttl               int
	forceNewCh        chan bool
	forceStopCh       chan bool
	services          []leaseMgr.Observer
}

type ServiceAccountKey struct {
	PrivateKeyID string `json:"private_key_id"`
}

func NewGCPLeaseManager(id, secretsPath string, client cvault.CVault) *GCPLeaseManager {
	return &GCPLeaseManager{
		id:          id,
		secretsPath: secretsPath,
		client:      client,
		forceNewCh:  make(chan bool, 1),
		forceStopCh: make(chan bool, 1),
	}
}

func (glm *GCPLeaseManager) GetNewLease() error {
	// client request new GCP credentials
	secrets, err := glm.client.Get(glm.secretsPath)
	if err != nil {
		glm.serviceAccountKey = []byte("")
		return err
	}

	if secrets == nil {
		return errors.New("Vault secret returns nil")
	}

	if secrets.Data == nil {
		return errors.New("Vault secret data is nil")
	}

	glm.ttl = secrets.LeaseDuration

	privateKeyDataIfc, ok := secrets.Data["private_key_data"]
	if !ok {
		return errors.New("Vault secret data is missing private_key_data")
	}

	privateKeyDataB64Str := privateKeyDataIfc.(string)

	privateKeyDataBytes, err := base64.StdEncoding.DecodeString(privateKeyDataB64Str)
	if err != nil {
		return err
	}

	sak := ServiceAccountKey{}
	if err := json.Unmarshal(privateKeyDataBytes, &sak); err != nil {
		return errors.Wrap(err, "failed to parse private key data from Vault")
	}
	log.Logger.Sugar().Infow("Retrieved service account key", "private_key_id", sak.PrivateKeyID)

	glm.serviceAccountKey = privateKeyDataBytes
	return nil
}

func (glm *GCPLeaseManager) NotifyNewLease() {
	glm.forceNewCh <- true
}

func (glm *GCPLeaseManager) NotifyStaleLease() {
	glm.serviceAccountKey = []byte("")
	glm.forceStopCh <- true
}

func (glm *GCPLeaseManager) GetID() string {
	return glm.id
}

func (glm *GCPLeaseManager) GetTTL() int {
	return glm.ttl
}

func (glm *GCPLeaseManager) GetServiceAccountKey() []byte {
	return glm.serviceAccountKey
}

func (glm *GCPLeaseManager) Daemonize(
	ctx context.Context,
	ctxCancelFunc context.CancelFunc,
	refreshPeriodInSecond time.Duration,
	earlyRenewalInMinute time.Duration,
) Daemon {
	return &daemon{
		gcpLeaseMgr:           glm,
		refreshPeriodInSecond: refreshPeriodInSecond,
		earlyRenewalInMinute:  earlyRenewalInMinute,
		ctx:                   ctx,
		ctxCancelFunc:         ctxCancelFunc,
		waitGroup:             sync.WaitGroup{},
		numRetry:              0,
		stopCh:                make(chan bool),
	}
}

func (glm *GCPLeaseManager) Register(service leaseMgr.Observer) {
	glm.services = append(glm.services, service)
}

func (glm *GCPLeaseManager) Deregister(childLease leaseMgr.Observer) {
	glm.services = leaseMgr.RemoveObserver(glm.services, childLease)
}

func (glm *GCPLeaseManager) NotifyAllNewLease() {
	for _, childLease := range glm.services {
		childLease.NotifyNewLease()
	}
}

func (glm *GCPLeaseManager) NotifyAllStaleLease() {
	for _, childLease := range glm.services {
		childLease.NotifyStaleLease()
	}
}
