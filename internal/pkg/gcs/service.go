package gcs

import (
	"context"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/mikeadityas/vault-gcs-lister/internal/pkg/gcp"
)

type BucketListerService struct {
	ctx           context.Context
	ctxCancelFunc context.CancelFunc
	id            string
	projectID     string
	forceNewCh    chan bool
	forceStopCh   chan bool
	gcpLeaseMgr   *gcp.GCPLeaseManager
}

func NewBucketListerService(
	ctx context.Context,
	ctxCancelFunc context.CancelFunc,
	id, projectID string,
	gcpLeaseMgr *gcp.GCPLeaseManager,
) *BucketListerService {
	return &BucketListerService{
		ctx:           ctx,
		ctxCancelFunc: ctxCancelFunc,
		id:            id,
		projectID:     projectID,
		forceNewCh:    make(chan bool, 1),
		forceStopCh:   make(chan bool, 1),
		gcpLeaseMgr:   gcpLeaseMgr,
	}
}

func (bls *BucketListerService) ListBucket() ([]string, error) {
	client, err := storage.NewClient(
		bls.ctx,
		option.WithCredentialsJSON(bls.gcpLeaseMgr.GetServiceAccountKey()),
	)
	if err != nil {
		return nil, err
	}

	var buckets []string
	bucketIter := client.Buckets(bls.ctx, bls.projectID)
	for {
		bucketAttributes, err := bucketIter.Next()
		if err == iterator.Done {
			break
		}

		if err != nil {
			return nil, err
		}
		buckets = append(buckets, bucketAttributes.Name)
	}

	return buckets, nil
}

func (bls *BucketListerService) NotifyNewLease() {
	bls.forceNewCh <- true
}

func (bls *BucketListerService) NotifyStaleLease() {
	bls.forceStopCh <- true
}

func (bls *BucketListerService) GetID() string {
	return bls.id
}

func (bls *BucketListerService) Daemonize(refreshPeriodInSecond time.Duration) Daemon {
	return &daemon{
		bucketListerSvc:              bls,
		desiredRefreshPeriodInSecond: refreshPeriodInSecond,
		currentRefreshPeriodInSecond: refreshPeriodInSecond,
		ctx:                          bls.ctx,
		ctxCancelFunc:                bls.ctxCancelFunc,
		waitGroup:                    sync.WaitGroup{},
		numRetry:                     0,
		stopCh:                       make(chan bool),
	}
}
