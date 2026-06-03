package isolator

import (
	"context"

	"github.com/coroot/coroot/model"
)

type MeshClient interface {
	RemoveFromMesh(ctx context.Context, app *model.Application) error
	RestoreToMesh(ctx context.Context, app *model.Application) error
	IsRemoved(ctx context.Context, app *model.Application) (bool, error)
}

type NoOpMeshClient struct{}

func (n *NoOpMeshClient) RemoveFromMesh(ctx context.Context, app *model.Application) error {
	return nil
}

func (n *NoOpMeshClient) RestoreToMesh(ctx context.Context, app *model.Application) error {
	return nil
}

func (n *NoOpMeshClient) IsRemoved(ctx context.Context, app *model.Application) (bool, error) {
	return false, nil
}