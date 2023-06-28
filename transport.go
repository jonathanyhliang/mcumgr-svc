package mcumgrsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	hawkbit "github.com/jonathanyhliang/hawkbit-fota/backend"
	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"

	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/ratelimit"
	"github.com/go-kit/kit/tracing/opentracing"
	httptransport "github.com/go-kit/kit/transport/http"

	stdopentracing "github.com/opentracing/opentracing-go"
)

// NewHTTPClient returns an AddService backed by an HTTP server living at the
// remote instance. We expect instance to come from a service discovery system,
// so likely of the form "host:port". We bake-in certain middlewares,
// implementing the client library pattern.
func NewHTTPClient(instance string, otTracer stdopentracing.Tracer, logger log.Logger) (IService, error) {
	// Quickly sanitize the instance string.
	if !strings.HasPrefix(instance, "http") {
		instance = "http://" + instance
	}
	u, err := url.Parse(instance)
	if err != nil {
		return nil, err
	}

	// We construct a single ratelimiter middleware, to limit the total outgoing
	// QPS from this client to all methods on the remote instance. We also
	// construct per-endpoint circuitbreaker middlewares to demonstrate how
	// that's done, although they could easily be combined into a single breaker
	// for the entire remote instance, too.
	limiter := ratelimit.NewErroringLimiter(rate.NewLimiter(rate.Every(time.Second), 100))

	// global client middlewares
	var options []httptransport.ClientOption

	// Each individual endpoint is an http/transport.Client (which implements
	// endpoint.Endpoint) that gets wrapped with various middlewares. If you
	// made your own client library, you'd do this work there, so your server
	// could rely on a consistent set of client behavior.
	var getControllerEndpoint endpoint.Endpoint
	{
		getControllerEndpoint = httptransport.NewClient(
			"GET",
			u,
			encodeGetControllerRequest,
			decodeGetControllerResponse,
			append(options, httptransport.ClientBefore(opentracing.ContextToHTTP(otTracer, logger)))...,
		).Endpoint()
		getControllerEndpoint = opentracing.TraceClient(otTracer, "GetController")(getControllerEndpoint)
		getControllerEndpoint = limiter(getControllerEndpoint)
		getControllerEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "GetController",
			Timeout: 30 * time.Second,
		}))(getControllerEndpoint)
	}
	var putConfigDataEndpoint endpoint.Endpoint
	{
		putConfigDataEndpoint = httptransport.NewClient(
			"PUT",
			u,
			encodePutConfigDataRequest,
			decodePutConfigDataResponse,
			append(options, httptransport.ClientBefore(opentracing.ContextToHTTP(otTracer, logger)))...,
		).Endpoint()
		putConfigDataEndpoint = opentracing.TraceClient(otTracer, "PutConfigData")(putConfigDataEndpoint)
		putConfigDataEndpoint = limiter(putConfigDataEndpoint)
		putConfigDataEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "PutConfigData",
			Timeout: 30 * time.Second,
		}))(putConfigDataEndpoint)
	}
	var getDeployBaseEndpoint endpoint.Endpoint
	{
		getDeployBaseEndpoint = httptransport.NewClient(
			"GET",
			u,
			encodeGetDeployBaseRequest,
			decodeGetDeployBaseResponse,
			append(options, httptransport.ClientBefore(opentracing.ContextToHTTP(otTracer, logger)))...,
		).Endpoint()
		getDeployBaseEndpoint = opentracing.TraceClient(otTracer, "GetDeployBase")(getDeployBaseEndpoint)
		getDeployBaseEndpoint = limiter(getDeployBaseEndpoint)
		getDeployBaseEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "GetDeployBase",
			Timeout: 30 * time.Second,
		}))(getDeployBaseEndpoint)
	}
	var postDeployBaseFeedbackEndpoint endpoint.Endpoint
	{
		postDeployBaseFeedbackEndpoint = httptransport.NewClient(
			"POST",
			u,
			encodePostDeployBaseFeedbackRequest,
			decodePostDeployBaseFeebackResponse,
			append(options, httptransport.ClientBefore(opentracing.ContextToHTTP(otTracer, logger)))...,
		).Endpoint()
		postDeployBaseFeedbackEndpoint =
			opentracing.TraceClient(otTracer, "PostDeployBaseFeedback")(postDeployBaseFeedbackEndpoint)
		postDeployBaseFeedbackEndpoint = limiter(postDeployBaseFeedbackEndpoint)
		postDeployBaseFeedbackEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "PostDeployBaseFeedback",
			Timeout: 30 * time.Second,
		}))(postDeployBaseFeedbackEndpoint)
	}
	var getDownloadHttpEndpoint endpoint.Endpoint
	{
		getDownloadHttpEndpoint = httptransport.NewClient(
			"GET",
			u,
			encodeGetDownloadHttpRequest,
			decodeGetDownloadHttpResponse,
			append(options, httptransport.ClientBefore(opentracing.ContextToHTTP(otTracer, logger)))...,
		).Endpoint()
		getDownloadHttpEndpoint =
			opentracing.TraceClient(otTracer, "GetDownloadHttp")(getDownloadHttpEndpoint)
		getDownloadHttpEndpoint = limiter(getDownloadHttpEndpoint)
		getDownloadHttpEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "GetDownloadHttp",
			Timeout: 30 * time.Second,
		}))(getDownloadHttpEndpoint)
	}

	// Returning the endpoint.Set as a Service relies on the
	// Endpoints implementing the Service methods. That's just a simple bit
	// of glue code.
	return Endpoints{
		GetControllerEndpoint:          getControllerEndpoint,
		PutConfigDataEndpoint:          putConfigDataEndpoint,
		GetDeployBaseEndpoint:          getDeployBaseEndpoint,
		PostDeployBaseFeedbackEndpoint: postDeployBaseFeedbackEndpoint,
		GetDownloadHttpEndpoint:        getDownloadHttpEndpoint,
	}, nil
}

func copyURL(base *url.URL, path string) *url.URL {
	next := *base
	next.Path = path
	return &next
}

func encodeGetControllerRequest(ctx context.Context, req *http.Request, request interface{}) error {
	// r.Methods("GET").Path("/default/controller/v1/{bid}")
	r := request.(hawkbit.GetControllerRequest)
	bid := url.QueryEscape(r.Bid)
	req.URL.Path = "/default/controller/v1/" + bid
	return encodeRequest(ctx, req, nil)
}

func encodePutConfigDataRequest(ctx context.Context, req *http.Request, request interface{}) error {
	// r.Methods("PUT").Path("/default/controller/v1/{bid}/configData")
	r := request.(hawkbit.PutConfigDataRequest)
	bid := url.QueryEscape(r.Bid)
	req.URL.Path = "/default/controller/v1/" + bid + "/configData"
	return encodeRequest(ctx, req, request)
}

func encodeGetDeployBaseRequest(ctx context.Context, req *http.Request, request interface{}) error {
	// r.Methods("GET").Path("/default/controller/v1/{bid}/deploymentBase/{acid}")
	r := request.(hawkbit.GetDeplymentBaseRequest)
	bid := url.QueryEscape(r.Bid)
	acid := url.QueryEscape(r.Acid)
	req.URL.Path = "/default/controller/v1/" + bid + "/deploymentBase/" + acid
	return encodeRequest(ctx, req, request)
}

func encodePostDeployBaseFeedbackRequest(ctx context.Context, req *http.Request, request interface{}) error {
	// r.Methods("POST").Path("/default/controller/v1/{bid}/deploymentBase/{acid}/feedback")
	r := request.(hawkbit.PostDeploymentBaseFeedbackRequest)
	bid := url.QueryEscape(r.Bid)
	acid := url.QueryEscape(r.Fb.ID)
	req.URL.Path = "/default/controller/v1/" + bid + "/deploymentBase/" + acid + "/feedback"
	return encodeRequest(ctx, req, request)
}

func encodeGetDownloadHttpRequest(ctx context.Context, req *http.Request, request interface{}) error {
	// r.Methods("GET").Path("/DEFAULT/controller/v1/{bid}/softwareModules/{ver}")
	r := request.(hawkbit.GetDownloadHttpRequest)
	bid := url.QueryEscape(r.Bid)
	ver := url.QueryEscape(r.Ver)
	req.URL.Path = "/DEFAULT/controller/v1/" + bid + "/softwareModules/" + ver
	return encodeRequest(ctx, req, request)
}

// encodeRequest likewise JSON-encodes the request to the HTTP request body.
// Don't use it directly as a transport/http.Client EncodeRequestFunc:
// profilesvc endpoints require mutating the HTTP method and request path.
func encodeRequest(_ context.Context, req *http.Request, request interface{}) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(request)
	if err != nil {
		return err
	}
	req.Body = ioutil.NopCloser(&buf)
	return nil
}

// decode?Response is a transport/http.DecodeResponseFunc that decodes a
// JSON-encoded sum response from the HTTP response body. If the response has a
// non-200 status code, we will interpret that as an error and attempt to decode
// the specific error message from the response body. Primarily useful in a
// client.
func decodeGetControllerResponse(_ context.Context, r *http.Response) (interface{}, error) {
	if r.StatusCode != http.StatusOK {
		return nil, errors.New(r.Status)
	}
	var resp hawkbit.GetControllerResponse
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

func decodePutConfigDataResponse(_ context.Context, r *http.Response) (interface{}, error) {
	if r.StatusCode != http.StatusOK {
		return nil, errors.New(r.Status)
	}
	var resp hawkbit.PutConfigDataResponse
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

func decodeGetDeployBaseResponse(_ context.Context, r *http.Response) (interface{}, error) {
	if r.StatusCode != http.StatusOK {
		return nil, errors.New(r.Status)
	}
	var resp hawkbit.GetDeplymentBaseResponse
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

func decodePostDeployBaseFeebackResponse(_ context.Context, r *http.Response) (interface{}, error) {
	if r.StatusCode != http.StatusOK {
		return nil, errors.New(r.Status)
	}
	var resp hawkbit.PostDeploymentBaseFeedbackResponse
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

func decodeGetDownloadHttpResponse(_ context.Context, r *http.Response) (interface{}, error) {
	if r.StatusCode != http.StatusOK {
		return nil, errors.New(r.Status)
	}
	var resp hawkbit.GetDownloadHttpResponse
	var err error
	resp.File, err = ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return resp, err
}
