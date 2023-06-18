package main

import (
	"context"

	hawkbit "github.com/jonathanyhliang/hawkbit-fota/backend"
)

type Service interface {
	GetController(ctx context.Context, bid string) (hawkbit.Controller, error)
	PutConfigData(ctx context.Context, bid string, cfg hawkbit.ConfigData) error
	GetDeployBase(ctx context.Context, bid, acid string) (hawkbit.DeploymentBase, error)
	PostDeployBaseFeedback(ctx context.Context, bid string, fb hawkbit.DeploymentBaseFeedback) error
	GetDownloadHttp(ctx context.Context, bid, ver string) []byte
}
