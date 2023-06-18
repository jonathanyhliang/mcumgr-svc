package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gopkg.in/cheggaaa/pb.v1"
	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/config"
	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmserial"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xact"
	"mynewt.apache.org/newtmgr/nmxact/xport"

	amqp "github.com/rabbitmq/amqp091-go"
)

// http://www.dest-unreach.org/socat/doc/socat-ttyovertcp.txt
// server (host)
// socat tcp-l:port,reuseaddr,fork file:/dev/tty,nonblock,raw,echo=0,crnl,waitlock=/var/run/tty
// clinet (svc)
// docker run -ti --rm --add-host host.docker.internal:host-gateway alpine:latest
// > socat pty,link=/dev/vmodem0,raw,echo=0,waitslave tcp:host.docker.internal:port

var (
	ErrBackendPortOpen  = errors.New("Backend: port open error")
	ErrBackendPortClose = errors.New("Backend: port close error")
	ErrBackendPortFlush = errors.New("Backend: port flush error")
	ErrBackendPort      = errors.New("Backend: invalid port setting")
	ErrBackendImage     = errors.New("Backend: invalid image")
	ErrBackendReset     = errors.New("Backend: failed to reset")
)

var globalSesn sesn.Sesn
var globalXport xport.Xport
var globalP *config.ConnProfile

// This keeps track of whether the global interface has been assigned.  This
// is necessary to accommodate golang's nil-interface semantics.
var globalXportSet bool
var globalTxFilter nmcoap.TxMsgFilter
var globalRxFilter nmcoap.RxMsgFilter

type Backend interface {
	Handler(port string, baud int, url string) error
	UploadImage(f []byte) error
	Reset()
	GetStatus() (exec, result string)
}

type mcumgrBackend struct {
	upld chan bool
	rst  chan bool
	ping chan bool
	img  []byte
	mtx  sync.Mutex
	sta  struct {
		exec   string
		result string
		mtx    sync.Mutex
	}
}

func NewMCUMgrBackend() Backend {
	return &mcumgrBackend{
		upld: make(chan bool),
		rst:  make(chan bool),
		ping: make(chan bool),
	}
}

func (b *mcumgrBackend) Handler(port string, baud int, url string) error {
	b.setStatus("closed", "none")
	args := make([]string, 3)
	args[0] = "acm"
	args[1] = "type=serial"
	args[2] = fmt.Sprintf("connstring=dev=%s,baud=%d,mtu=512", port, baud)
	err := connProfileAddCmd(args)
	if err != nil {
		return err
	}

	b.mtx.Lock()

	go func() {
		b.msgQueueReceive(url)
	}()

	for {
		select {
		case <-b.upld:
			b.setStatus("proceeding", "none")
			// err = imageUploadCmd(b.img)
			b.setStatus("downloaded", "success")

		case <-b.rst:
			err = resetRunCmd([]string{})
			if err != nil {

			}
			// Close opening serial port
			time.Sleep(3 * time.Second)
			cleanup()
			b.mtx.Unlock()

		case <-b.ping:
			// Establish serial connection as soon as pinged by SLCAN service
			fmt.Println("unlock")
			b.mtx.Unlock()

		default:
		}
	}
}

func (b *mcumgrBackend) UploadImage(f []byte) error {
	if f == nil {
		return ErrBackendImage
	}
	fmt.Println("UploadImage")
	if b.mtx.TryLock() {
		fmt.Println("TryLock")
		b.setStatus("scheduled", "none")
		b.img = f
		b.upld <- true
	}

	return nil
}

func (b *mcumgrBackend) Reset() {
	b.rst <- true
	return
}

func (b *mcumgrBackend) setStatus(exec, result string) {
	b.sta.mtx.Lock()
	b.sta.exec = exec
	b.sta.result = result
	defer b.sta.mtx.Unlock()
}

func (b *mcumgrBackend) GetStatus() (exec, result string) {
	b.sta.mtx.Lock()
	defer b.sta.mtx.Unlock()
	return b.sta.exec, b.sta.result
}

func (b *mcumgrBackend) msgQueueReceive(url string) error {
	conn, err := amqp.Dial(url)
	if err != nil {

	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {

	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"handover", // name
		false,      // durable
		false,      // delete when unused
		false,      // exclusive
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {

	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {

	}

	for {
		for d := range msgs {
			if d.Body != nil {
				b.ping <- true
			}
		}
	}
}

func connProfileAddCmd(args []string) error {
	// Connection Profile name required
	if len(args) == 0 {
		return ErrBackendPort
	}

	name := args[0]
	cp := config.NewConnProfile()
	cp.Name = name
	cp.Type = config.CONN_TYPE_NONE

	for _, vdef := range args[1:] {
		s := strings.SplitN(vdef, "=", 2)
		switch s[0] {
		case "type":
			var err error
			cp.Type, err = config.ConnTypeFromString(s[1])
			if err != nil {
				return ErrBackendPort
			}
		case "connstring":
			cp.ConnString = s[1]
		default:
			return ErrBackendPort
		}
	}

	// Check that a type is specified.
	if cp.Type == config.CONN_TYPE_NONE {
		return ErrBackendPort
	}

	globalP = cp

	return nil
}

func imageUploadCmd(img []byte) error {
	noerase := false
	imageNum := 0
	upgrade := false
	maxWinSz := xact.IMAGE_UPLOAD_DEF_MAX_WS

	s, err := getSesn()
	if err != nil {
		return ErrBackendImage
	}

	c := xact.NewImageUpgradeCmd()
	var opt = sesn.TxOptions{
		Timeout: time.Duration(5 * float64(time.Second)),
		Tries:   2,
	}
	c.SetTxOptions(opt)
	c.Data = img
	if noerase == true {
		c.NoErase = true
	}
	if imageNum < 0 {
		return ErrBackendImage
	}
	c.ImageNum = imageNum
	c.Upgrade = upgrade
	c.ProgressBar = pb.StartNew(len(img))
	c.ProgressBar.SetUnits(pb.U_BYTES)
	c.ProgressBar.ShowSpeed = true
	c.LastOff = 0
	c.MaxWinSz = maxWinSz
	c.ProgressCb = func(cmd *xact.ImageUploadCmd, rsp *nmp.ImageUploadRsp) {
		if rsp.Off > c.LastOff {
			c.ProgressBar.Add(int(rsp.Off - c.LastOff))
			c.LastOff = rsp.Off
		}
	}

	res, err := c.Run(s)
	if err != nil {
		return err
	}

	if res.Status() != 0 {
		return ErrBackendImage
	}

	c.ProgressBar.Finish()

	return nil
}

func resetRunCmd(args []string) error {
	s, err := getSesn()
	if err != nil {
		return ErrBackendReset
	}

	c := xact.NewResetCmd()
	var opt = sesn.TxOptions{
		Timeout: time.Duration(5 * float64(time.Second)),
		Tries:   2,
	}
	c.SetTxOptions(opt)

	if _, err := c.Run(s); err != nil {
		return ErrBackendReset
	}

	return nil
}

func getSesn() (sesn.Sesn, error) {
	if globalSesn != nil {
		return globalSesn, nil
	}

	var s sesn.Sesn

	sc, err := buildSesnCfg()
	if err != nil {
		return nil, err
	}
	sc.TxFilter = globalTxFilter
	sc.RxFilter = globalRxFilter

	x, err := getXport()
	if err != nil {
		return nil, err
	}

	s, err = x.BuildSesn(sc)
	if err != nil {
		return nil, util.ChildNewtError(err)
	}

	globalSesn = s
	if err := globalSesn.Open(); err != nil {
		return nil, util.ChildNewtError(err)
	}

	return globalSesn, nil
}

func getXport() (xport.Xport, error) {
	if globalXport != nil {
		return globalXport, nil
	}

	cp := globalP
	switch cp.Type {
	case config.CONN_TYPE_SERIAL_PLAIN, config.CONN_TYPE_SERIAL_OIC:
		sc, err := config.ParseSerialConnString(cp.ConnString)
		if err != nil {
			return nil, err
		}

		globalXport = nmserial.NewSerialXport(sc)
	default:
		return nil, util.FmtNewtError("Unknown connection type: %s (%d)",
			config.ConnTypeToString(cp.Type), int(cp.Type))
	}

	globalXportSet = true

	if err := globalXport.Start(); err != nil {
		return nil, util.ChildNewtError(err)
	}

	return globalXport, nil
}

func buildSesnCfg() (sesn.SesnCfg, error) {
	sc := sesn.NewSesnCfg()
	cp := globalP
	switch cp.Type {
	case config.CONN_TYPE_SERIAL_PLAIN:
		sc.MgmtProto = sesn.MGMT_PROTO_NMP
		return sc, nil
	default:
		return sc, util.FmtNewtError("Unknown connection type: %s (%d)",
			config.ConnTypeToString(cp.Type), int(cp.Type))
	}
}

func stopXport() {
	if globalXportSet {
		// Don't attempt to close a serial transport.  Attempting to close
		// the serial port while a read is in progress (in MacOS) just
		// blocks until the read completes.  Instead, let the OS close the
		// port on termination.
		globalXport.Stop()
	}
}

func closeSesn() {
	if globalSesn != nil {
		globalSesn.Close()
	}
}

func cleanup() {
	closeSesn()
	stopXport()
}
