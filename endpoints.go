package main

import (
	"context"

	hawkbit "github.com/jonathanyhliang/hawkbit-fota/backend"

	"github.com/go-kit/kit/endpoint"
)

type Endpoints struct {
	GetControllerEndpoint          endpoint.Endpoint
	PutConfigDataEndpoint          endpoint.Endpoint
	GetDeployBaseEndpoint          endpoint.Endpoint
	PostDeployBaseFeedbackEndpoint endpoint.Endpoint
	GetDownloadHttpEndpoint        endpoint.Endpoint
}

func (e Endpoints) GetController(ctx context.Context, bid string) (hawkbit.Controller, error) {
	resp, err := e.GetControllerEndpoint(ctx, hawkbit.GetControllerRequest{Bid: bid})
	if err != nil {
		return hawkbit.Controller{}, err
	}
	response := resp.(hawkbit.GetControllerResponse)
	return response.Ctrlr, response.Err
}

func (e Endpoints) PutConfigData(ctx context.Context, bid string, cfg hawkbit.ConfigData) error {
	resp, err := e.PutConfigDataEndpoint(ctx, hawkbit.PutConfigDataRequest{Bid: bid, Cfg: cfg})
	if err != nil {
		return err
	}
	response := resp.(hawkbit.PutConfigDataResponse)
	return response.Err
}

func (e Endpoints) GetDeployBase(ctx context.Context, bid, acid string) (hawkbit.DeploymentBase, error) {
	resp, err := e.GetDeployBaseEndpoint(ctx, hawkbit.GetDeplymentBaseRequest{Bid: bid, Acid: acid})
	if err != nil {
		return hawkbit.DeploymentBase{}, nil
	}
	response := resp.(hawkbit.GetDeplymentBaseResponse)
	return response.Dp, response.Err
}

func (e Endpoints) PostDeployBaseFeedback(ctx context.Context, bid string, fb hawkbit.DeploymentBaseFeedback) error {
	resp, err := e.PostDeployBaseFeedbackEndpoint(ctx, hawkbit.PostDeploymentBaseFeedbackRequest{Bid: bid, Fb: fb})
	if err != nil {
		return err
	}
	response := resp.(hawkbit.PostDeploymentBaseFeedbackResponse)
	return response.Err
}

func (e Endpoints) GetDownloadHttp(ctx context.Context, bid, ver string) []byte {
	resp, err := e.GetDownloadHttpEndpoint(ctx, hawkbit.GetDownloadHttpRequest{Bid: bid, Ver: ver})
	if err != nil {
		return nil
	}
	response := resp.(hawkbit.GetDownloadHttpResponse)
	return response.File
}
