package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"gopkg.in/cheggaaa/pb.v1"
	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/cli"
	"mynewt.apache.org/newtmgr/newtmgr/config"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmserial"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xact"
	"mynewt.apache.org/newtmgr/nmxact/xport"
)

var (
	ErrBackendPort  = errors.New("Backend: invalid port setting")
	ErrBackendImage = errors.New("Backend: invalid image")
	ErrBackendReset = errors.New("Backend: failed to reset")
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
	Handler(port string, baud int) error
	UploadImage(f string)
	GetStatus() Status
	Reset()
}

type mcumgrBackend struct {
	upld     chan string
	rst      chan bool
	mtx      sync.Mutex
	sta      Status
	handover bool
}

func NewMCUMgrBackend() Backend {
	return &mcumgrBackend{
		upld: make(chan string),
		rst:  make(chan bool),
	}
}

func (m *mcumgrBackend) Handler(port string, baud int) error {
	args := make([]string, 3)
	args[0] = "acm"
	args[1] = "type=serial"
	args[2] = fmt.Sprintf("connstring=dev=%s,baud=%d,mtu=512", port, baud)
	if err := connProfileAddCmd(args); err != nil {
		return err
	}

	for {
		select {
		case f := <-m.upld:
			err := imageUploadCmd([]string{f})
			m.mtx.Lock()
			if err != nil {
				m.sta.Execution = "canceled"
				m.sta.Result.Finished = "failure"
			} else {
				m.sta.Execution = "downloaded"
				m.sta.Result.Finished = "success"
			}
			m.mtx.Unlock()

		case <-m.rst:
			err := resetRunCmd([]string{})
			if err != nil {

			}
		default:
		}
	}
}

func (m *mcumgrBackend) UploadImage(f string) {
	m.mtx.Lock()
	m.sta.Execution = "download"
	m.sta.Result.Finished = "none"
	m.mtx.Unlock()
	m.upld <- f
	return
}

func (m *mcumgrBackend) GetStatus() Status {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	return m.sta
}

func (m *mcumgrBackend) Reset() {
	m.rst <- true
	return
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

func imageUploadCmd(args []string) error {
	noerase := false
	imageNum := 0
	upgrade := false
	maxWinSz := xact.IMAGE_UPLOAD_DEF_MAX_WS

	if len(args) < 1 {
		return ErrBackendImage
	}

	imageFile, err := ioutil.ReadFile(args[0])
	if err != nil {
		return ErrBackendImage
	}

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
	c.Data = imageFile
	if noerase == true {
		c.NoErase = true
	}
	if imageNum < 0 {
		return ErrBackendImage
	}
	c.ImageNum = imageNum
	c.Upgrade = upgrade
	c.ProgressBar = pb.StartNew(len(imageFile))
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
	s, err := cli.GetSesn()
	if err != nil {
		return ErrBackendReset
	}

	c := xact.NewResetCmd()
	c.SetTxOptions(nmutil.TxOptions())

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
