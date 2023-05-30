package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/log"
)

func main() {
	var (
		httpAddr = flag.String("a", ":8081", "HTTP listen address")
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

	var b Backend
	{
		b = NewMCUMgrBackend()
	}

	var s Service
	{
		s = NewMcumgrService()
		s = BackendMiddleware(b)(s)
		s = LoggingMiddleware(logger)(s)
	}

	var h http.Handler
	{
		h = MakeHTTPHandler(s, log.With(logger, "component", "HTTP"))
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
		logger.Log("transport", "HTTP", "addr", *httpAddr)
		errs <- http.ListenAndServe(*httpAddr, h)
	}()

	logger.Log("exit", <-errs)
}
