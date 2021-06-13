/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package network

import (
	"context"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"
	"time"

	"mosn.io/pkg/buffer"

	"mosn.io/api"
)

type MyEventListener struct{}

func (el *MyEventListener) OnEvent(event api.ConnectionEvent) {}

func testAddConnectionEventListener(n int, t *testing.T) {
	c := connection{}

	for i := 0; i < n; i++ {
		el0 := &MyEventListener{}
		c.AddConnectionEventListener(el0)
	}

	if len(c.connCallbacks) != n {
		t.Errorf("Expect %d, but got %d after AddConnectionEventListener(el0)", n, len(c.connCallbacks))
	}
}

func testAddBytesReadListener(n int, t *testing.T) {
	c := connection{}

	for i := 0; i < n; i++ {
		fn1 := func(bytesRead uint64) {}
		c.AddBytesReadListener(fn1)
	}

	if len(c.bytesReadCallbacks) != n {
		t.Errorf("Expect %d, but got %d after AddBytesReadListener(fn1)", n, len(c.bytesReadCallbacks))
	}
}

func testAddBytesSendListener(n int, t *testing.T) {
	c := connection{}

	for i := 0; i < n; i++ {
		fn1 := func(bytesSent uint64) {}
		c.AddBytesSentListener(fn1)
	}

	if len(c.bytesSendCallbacks) != n {
		t.Errorf("Expect %d, but got %d after AddBytesSentListener(fn1)", n, len(c.bytesSendCallbacks))
	}
}

func TestAddConnectionEventListener(t *testing.T) {
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("AddConnectionEventListener(%d)", i)
		t.Run(name, func(t *testing.T) {
			testAddConnectionEventListener(i, t)
		})
	}
}

func TestAddBytesReadListener(t *testing.T) {
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("AddBytesReadListener(%d)", i)
		t.Run(name, func(t *testing.T) {
			testAddBytesReadListener(i, t)
		})
	}
}

func TestAddBytesSendListener(t *testing.T) {
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("AddBytesSendListener(%d)", i)
		t.Run(name, func(t *testing.T) {
			testAddBytesSendListener(i, t)
		})
	}
}

func TestClientConectionRemoteaddrIsNil(t *testing.T) {
	conn := NewClientConnection(0, nil, nil, nil)
	err := conn.Connect()
	if err == nil {
		t.Errorf("connect should Failed")
		return
	}
}

type zeroReadConn struct {
	net.Conn
}

func (r *zeroReadConn) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (r *zeroReadConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (r *zeroReadConn) LocalAddr() net.Addr {
	return nil
}

func TestIoBufferZeroRead(t *testing.T) {
	conn := &connection{}
	conn.rawConnection = &zeroReadConn{}
	err := conn.doRead()
	if err != io.EOF {
		t.Errorf("error should be io.EOF")
	}
}

func testConnStateBase(addr net.Addr, t *testing.T) {
	l, err := net.Listen(addr.Network(), addr.String())
	if err != nil {
		t.Logf("listen error %v", err)
		return
	}
	rawc, err := net.Dial(addr.Network(), addr.String())
	if err != nil {
		t.Logf("net.Dial error %v", err)
		return
	}
	c := NewServerConnection(context.Background(), rawc, nil)
	if c.State() != api.ConnActive {
		t.Errorf("ConnState should be ConnActive")
	}
	c.Close(api.NoFlush, api.LocalClose)
	if c.State() != api.ConnClosed {
		t.Errorf("ConnState should be ConnClosed")
	}

	cc := NewClientConnection(0, nil, addr, nil)
	if cc.State() != api.ConnInit {
		t.Errorf("ConnState should be ConnInit")
	}
	if err := cc.Connect(); err != nil {
		t.Errorf("conn Connect error: %v", err)
	}
	if cc.State() != api.ConnActive {
		t.Errorf("ConnState should be ConnActive")
	}
	cc.Write()
	l.Close()

	time.Sleep(10 * time.Millisecond)
	if cc.State() != api.ConnClosed {
		t.Errorf("ConnState should be ConnClosed")
	}
}

func TestTCPConnectState(t *testing.T) {
	testAddr := "127.0.0.1:11234"
	remoteAddr, _ := net.ResolveTCPAddr("tcp", testAddr)
	testConnStateBase(remoteAddr, t)

}
func TestUDSConnectState(t *testing.T) {
	addr, _ := net.ResolveUnixAddr("unix", "/tmp/test.sock")
	testConnStateBase(addr, t)
}

func TestUDSWriteRead(t *testing.T) {
	addr, _ := net.ResolveUnixAddr("unix", "/tmp/test1.sock")
	syscall.Unlink(addr.String())
	l, err := net.Listen(addr.Network(), addr.String())
	if err != nil {
		t.Fatalf("listen error %v", err)
	}
	defer l.Close()

	go func() {
		conn, _ := l.Accept()
		read := make([]byte, 1024)
		i, _ := conn.Read(read)
		if i <= 0 {
			t.Fatalf("conn read error: %v", err)
		}
		if _, err = conn.Write([]byte("hello,client")); err != nil {
			t.Fatalf("conn Write error: %v", err)
		}

	}()
	time.Sleep(time.Second) // wait accept goroutine

	cc := NewClientConnection(0, nil, addr, nil)
	defer cc.Close(api.FlushWrite, api.RemoteClose)
	// add read filter
	filter := &testReadFilter{}
	cc.FilterManager().AddReadFilter(filter)
	if err := cc.Connect(); err != nil {
		t.Fatalf("conn Connect error: %v", err)
	}
	if cc.State() != api.ConnActive {
		t.Fatalf("ConnState should be ConnActive")
	}
	wb := buffer.GetIoBuffer(10)
	wb.WriteString("hello,mosn")
	if err := cc.Write(wb); err != nil {
		t.Fatalf("conn WriteString error: %v", err)
	}
	time.Sleep(time.Millisecond * 500)
	if filter.received <= 0 {
		t.Fatalf("conn can not received server's msg")
	}
}

type testReadFilter struct {
	received int
}

func (t *testReadFilter) OnData(buffer buffer.IoBuffer) api.FilterStatus {
	t.received += buffer.Len()
	return ""
}

func (t *testReadFilter) OnNewConnection() api.FilterStatus {
	return ""

}

func (t *testReadFilter) InitializeReadFilterCallbacks(cb api.ReadFilterCallbacks) {
}
