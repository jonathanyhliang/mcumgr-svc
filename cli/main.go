package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	hawkbit "github.com/jonathanyhliang/hawkbit-fota/backend"
	mcumgrsvc "github.com/jonathanyhliang/mcumgr-svc"
	stdopentracing "github.com/opentracing/opentracing-go"
)

func main() {
	var (
		bid      = flag.String("bid", "", "Board ID")
		httpAddr = flag.String("s", "", "HTTP address of addsvc")
		amqpURL  = flag.String("u", "amqp://guest:guest@localhost:5672/", "AMQP dialing address")
		port     = flag.String("p", "", "MCUMgr port")
		baud     = flag.Int("b", 115200, "MCUMgr port baudrate")
	)
	flag.Parse()

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	var b mcumgrsvc.Backend
	{
		b = mcumgrsvc.NewMCUMgrBackend()
	}

	// This is a demonstration client, which supports multiple tracers.
	// Your clients will probably just use one tracer.
	var otTracer stdopentracing.Tracer
	{
		otTracer = stdopentracing.GlobalTracer() // no-op
	}

	var svc mcumgrsvc.IService
	{
		var err error
		svc, err = mcumgrsvc.NewHTTPClient(*httpAddr, otTracer, log.NewNopLogger())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		svc = mcumgrsvc.LoggingMiddleware(logger)(svc)
	}

	errs := make(chan error)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("%s", <-c)
	}()

	go func() {
		errs <- b.Handler(*port, *baud, *amqpURL)
	}()

	go func() {
		var ctrlr hawkbit.Controller
		var cfgData hawkbit.ConfigData
		var deployBase hawkbit.DeploymentBase
		var deployBaseFdbk hawkbit.DeploymentBaseFeedback
		var acid string = ""
		var err error

		for {
			ctrlr, err = svc.GetController(context.Background(), *bid)
			if err != nil {
				errs <- err
			}

			if ctrlr.Links.ConfigData.Href != "" {
				cfgData.Data.HwRevision = "01"
				cfgData.Data.VIN = *bid
				cfgData.Mode = " merge"
				err = svc.PutConfigData(context.Background(), *bid, cfgData)
				if err != nil {
					errs <- err
				}
			}

			if ctrlr.Links.DeploymentBase.Href != "" {
				_, _acid := parseDeployBsaeHref(ctrlr.Links.DeploymentBase.Href)
				if _acid != acid {
					deployBase, err = svc.GetDeployBase(context.Background(), *bid, _acid)
					if err != nil {
						errs <- err
					}

					if f := deployBase.Deployment.Chunks[0].Artifacts[0].Links.DownloadHttp.Href; f != "" {
						_, ver := parseDownloadHttpHref(f)
						err := b.UploadImage(svc.GetDownloadHttp(context.Background(),
							*bid, ver))
						if err != nil {
							errs <- err
						}

						for {
							exec, result := b.GetStatus()
							if exec == "closed" {
								break
							}
							if result != "none" {
								acid = _acid
								b.Reset()
								break
							}
						}
					}

					deployBaseFdbk.ID = _acid
					deployBaseFdbk.Status.Execution, deployBaseFdbk.Status.Result.Finished = b.GetStatus()
					err = svc.PostDeployBaseFeedback(context.Background(), *bid, deployBaseFdbk)
				}
			}

			time.Sleep(parseSleepTime(ctrlr.Config.Polling.Sleep))
		}
	}()

	logger.Log("exit", <-errs)
}

func parseSleepTime(t string) time.Duration {
	sleep := time.Duration(2) * time.Minute
	n := strings.Split(t, ":")
	if len(n) != 3 {
		return sleep
	}
	ss, err := strconv.Atoi(n[2])
	if err != nil {
		return sleep
	}
	mm, err := strconv.Atoi(n[1])
	if err != nil {
		return sleep
	}
	hh, err := strconv.Atoi(n[0])
	if err != nil {
		return sleep
	}
	sleep = time.Duration(hh*3600+mm*60+ss) * time.Second
	return sleep
}

func parseDeployBsaeHref(u string) (bid, acid string) {
	n := strings.Split(u, "/")
	if len(n) < 7 {
		return "", ""
	}
	return n[4], n[6]
}

func parseDownloadHttpHref(u string) (bid, ver string) {
	n := strings.Split(u, "/")
	if len(n) < 7 {
		return "", ""
	}
	return n[4], n[6]
}
