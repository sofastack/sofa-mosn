package integrate

import (
	"context"
	"testing"
	"time"

	"sofastack.io/sofa-mosn/pkg/module/http2"
	"sofastack.io/sofa-mosn/pkg/mosn"
	"sofastack.io/sofa-mosn/pkg/protocol"
	_ "sofastack.io/sofa-mosn/pkg/protocol/rpc/sofarpc/codec"
	_ "sofastack.io/sofa-mosn/pkg/protocol/rpc/sofarpc/conv"
	"sofastack.io/sofa-mosn/pkg/stream"
	_ "sofastack.io/sofa-mosn/pkg/stream/http"
	_ "sofastack.io/sofa-mosn/pkg/stream/http2"
	_ "sofastack.io/sofa-mosn/pkg/stream/sofarpc"
	_ "sofastack.io/sofa-mosn/pkg/stream/xprotocol"
	"sofastack.io/sofa-mosn/pkg/types"
	"sofastack.io/sofa-mosn/test/util"
)

func (c *TestCase) StartAuto(tls bool) {
	c.AppServer.GoServe()
	appAddr := c.AppServer.Addr()
	clientMeshAddr := util.CurrentMeshAddr()
	c.ClientMeshAddr = clientMeshAddr
	serverMeshAddr := util.CurrentMeshAddr()
	cfg := util.CreateMeshToMeshConfig(clientMeshAddr, serverMeshAddr, protocol.Auto, protocol.Auto, []string{appAddr}, tls)
	mesh := mosn.NewMosn(cfg)
	go mesh.Start()
	go c.DeferFinishCase(func() {
		c.AppServer.Close()
		mesh.Close()
	})
	time.Sleep(5 * time.Second) //wait server and mesh start
}

func TestAuto(t *testing.T) {
	appaddr := "127.0.0.1:8080"
	testCases := []*TestCase{
		NewTestCase(t, protocol.HTTP2, protocol.HTTP2, util.NewUpstreamHTTP2(t, appaddr, nil)),
		NewTestCase(t, protocol.HTTP1, protocol.HTTP1, util.NewHTTPServer(t, nil)),
	}
	for i, tc := range testCases {
		t.Logf("start case #%d\n", i)
		tc.StartAuto(false)
		go tc.RunCase(5, 0)
		select {
		case err := <-tc.C:
			if err != nil {
				t.Errorf("[ERROR MESSAGE] #%d %v to mesh %v test failed, error: %v\n", i, tc.AppProtocol, tc.MeshProtocol, err)
			}
		case <-time.After(15 * time.Second):
			t.Errorf("[ERROR MESSAGE] #%d %v to mesh %v hang\n", i, tc.AppProtocol, tc.MeshProtocol)
		}
		tc.FinishCase()
	}
}

func TestAutoTLS(t *testing.T) {
	appaddr := "127.0.0.1:8080"
	testCases := []*TestCase{
		NewTestCase(t, protocol.HTTP2, protocol.HTTP2, util.NewUpstreamHTTP2(t, appaddr, nil)),
		NewTestCase(t, protocol.HTTP1, protocol.HTTP1, util.NewHTTPServer(t, nil)),
	}
	for i, tc := range testCases {
		t.Logf("start case #%d\n", i)
		tc.StartAuto(true)
		go tc.RunCase(5, 0)
		select {
		case err := <-tc.C:
			if err != nil {
				t.Errorf("[ERROR MESSAGE] #%d %v to mesh %v test failed, error: %v\n", i, tc.AppProtocol, tc.MeshProtocol, err)
			}
		case <-time.After(15 * time.Second):
			t.Errorf("[ERROR MESSAGE] #%d %v to mesh %v hang\n", i, tc.AppProtocol, tc.MeshProtocol)
		}
		tc.FinishCase()
	}
}

func TestProtocolHttp2(t *testing.T) {
	var prot types.Protocol
	var magic string
	var err error

	magic = http2.ClientPreface
	prot, err = stream.SelectStreamFactoryProtocol(context.Background(), "", []byte(magic))
	if prot != protocol.HTTP2 {
		t.Errorf("[ERROR MESSAGE] type error magic : %v\n", magic)
	}

	len := len(http2.ClientPreface)
	prot, err = stream.SelectStreamFactoryProtocol(context.Background(), "", []byte(magic)[0:len-1])
	if err != stream.EAGAIN {
		t.Errorf("[ERROR MESSAGE] type error protocol :%v", err)
	}

	prot, err = stream.SelectStreamFactoryProtocol(context.Background(), "", []byte("helloworld"))
	if err != stream.FAILED {
		t.Errorf("[ERROR MESSAGE] type error protocol :%v", err)
	}
}

func TestProtocolHttp1(t *testing.T) {
	var prot types.Protocol
	var magic string
	var err error

	magic = "GET"
	prot, err = stream.SelectStreamFactoryProtocol(nil, "", []byte(magic))
	if prot != protocol.HTTP1 {
		t.Errorf("[ERROR MESSAGE] type error magic : %v\n", magic)
	}

	magic = "POST"
	prot, err = stream.SelectStreamFactoryProtocol(nil, "", []byte(magic))
	if prot != protocol.HTTP1 {
		t.Errorf("[ERROR MESSAGE] type error magic : %v\n", magic)
	}

	magic = "POS"
	prot, err = stream.SelectStreamFactoryProtocol(nil, "", []byte(magic))
	if err != stream.EAGAIN {
		t.Errorf("[ERROR MESSAGE] type error protocol :%v", err)
	}

	magic = "PPPPPPP"
	prot, err = stream.SelectStreamFactoryProtocol(nil, "", []byte(magic))
	if err != stream.FAILED {
		t.Errorf("[ERROR MESSAGE] type error protocol :%v", err)
	}
}
