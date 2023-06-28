package mcumgrsvc

import (
	"context"
	"time"

	"github.com/go-kit/kit/log"
	hawkbit "github.com/jonathanyhliang/hawkbit-fota/backend"
)

// Middleware describes a service (as opposed to endpoint) middleware.
type Middleware func(IService) IService

func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next IService) IService {
		return &loggingMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type loggingMiddleware struct {
	next   IService
	logger log.Logger
}

func (mw loggingMiddleware) GetController(ctx context.Context, bid string) (ctrlr hawkbit.Controller, err error) {
	defer func(begin time.Time) {
		mw.logger.Log("method", "GetController", "bid", bid, "took", time.Since(begin), "err", err)
	}(time.Now())
	return mw.next.GetController(ctx, bid)
}

func (mw loggingMiddleware) PutConfigData(ctx context.Context, bid string, cfg hawkbit.ConfigData) (err error) {
	defer func(begin time.Time) {
		mw.logger.Log("method", "PutConfigData", "bid", bid, "took", time.Since(begin), "err", err)
	}(time.Now())
	return mw.next.PutConfigData(ctx, bid, cfg)
}

func (mw loggingMiddleware) GetDeployBase(ctx context.Context, bid, acid string) (dp hawkbit.DeploymentBase, err error) {
	defer func(begin time.Time) {
		mw.logger.Log("method", "GetDeployBase", "bid", bid, "acid", acid, "took", time.Since(begin), "err", err)
	}(time.Now())
	return mw.next.GetDeployBase(ctx, bid, acid)
}

func (mw loggingMiddleware) PostDeployBaseFeedback(ctx context.Context, bid string,
	fb hawkbit.DeploymentBaseFeedback) (err error) {
	defer func(begin time.Time) {
		mw.logger.Log("method", "PostDeployBaseFeedback", "bid", bid, "took", time.Since(begin), "err", err)
	}(time.Now())
	return mw.next.PostDeployBaseFeedback(ctx, bid, fb)
}

func (mw loggingMiddleware) GetDownloadHttp(ctx context.Context, bid, ver string) []byte {
	defer func(begin time.Time) {
		mw.logger.Log("method", "GetDownloadHttp", "bid", bid, "ver", ver, "took", time.Since(begin))
	}(time.Now())
	return mw.next.GetDownloadHttp(ctx, bid, ver)
}
