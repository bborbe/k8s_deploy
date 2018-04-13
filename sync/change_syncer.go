package sync

import (
	"context"

	"github.com/bborbe/run"
	"github.com/golang/glog"
	"github.com/seibert-media/k8s-deploy/change"
)

const channelSize = 10

type applyChanges func(context.Context, <-chan change.Change) error
type getChanges func(context.Context, chan<- change.Change) error

// Syncer is responsible for sending incoming changes to the apply function
type Syncer interface {
	SyncChanges(ctx context.Context) error
}

type syncer struct {
	getChanges   getChanges
	applyChanges applyChanges
}

// New Syncer taking get and apply functions
func New(
	getChanges getChanges,
	applyChanges applyChanges,
) Syncer {
	return &syncer{
		getChanges:   getChanges,
		applyChanges: applyChanges,
	}
}

// SyncChanges until one function errors
func (c *syncer) SyncChanges(ctx context.Context) error {
	glog.V(1).Info("sync changes started")
	defer glog.V(1).Info("sync changes finished")
	versionChannel := make(chan change.Change, channelSize)

	return run.CancelOnFirstError(ctx,
		// get changes
		func(ctx context.Context) error {
			return c.getChanges(ctx, versionChannel)
		},
		// apply changes
		func(ctx context.Context) error {
			return c.applyChanges(ctx, versionChannel)
		},
	)
}
