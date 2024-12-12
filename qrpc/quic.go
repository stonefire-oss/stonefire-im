package qrpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/stonefire-oss/stonefire-im/pkg/codec"
	"github.com/stonefire-oss/stonefire-im/pkg/utils"
)

const (
	NoError               = 0
	SessionTimeoutErr     = 0xFF00
	UnSupportMessageErr   = 0xFF01
	ServiceUnavailableErr = 0xFF02

	SessionTimeoutErrMsg     = "session timeout"
	UnSupportMessageErrMsg   = "unsupport message type"
	ServiceUnavailableErrMsg = "service unavailable"
)

type closeReason uint64

func (r closeReason) code() quic.ApplicationErrorCode {
	return quic.ApplicationErrorCode(r)
}

func (r closeReason) String() string {
	switch r {
	case NoError:
		return "no error"
	case SessionTimeoutErr:
		return "session timeout"
	case UnSupportMessageErr:
		return "unsupport message type"
	case ServiceUnavailableErr:
		return "service unavailable"
	default:
		return fmt.Sprintf("unknown code %d", r)
	}
}

type qrpcConn struct {
	conn              quic.Connection
	quit              *utils.Event
	handler           streamHandler
	closed            *utils.Event
	mu                sync.Mutex
	idle              time.Time
	maxConnectionIdle time.Duration
	ctx               context.Context
	plmk              codec.PayloadBuilder
}

func newQRPConn(conn quic.Connection, s *Server, handler streamHandler) *qrpcConn {
	qc := &qrpcConn{
		conn:              conn,
		quit:              s.quit,
		handler:           handler,
		closed:            utils.NewEvent(),
		maxConnectionIdle: s.opts.maxConnectionIdle,
		ctx:               context.Background(),
		plmk:              utils.NewPooledPlMaker(s.opts.bufferPool),
	}
	return qc
}

func (c *qrpcConn) close(r closeReason) error {
	c.mu.Lock()

	defer func() {
		c.mu.Unlock()
	}()

	if c.closed.Fire() {
		return c.conn.CloseWithError(r.code(), r.String())
	}
	return nil
}

func (c *qrpcConn) keepalive() {
	idleTimer := time.NewTimer(c.maxConnectionIdle)

	defer func() {
		idleTimer.Stop()
	}()

	for {
		select {
		case <-idleTimer.C:
			c.mu.Lock()
			idle := c.idle
			if idle.IsZero() {
				c.mu.Unlock()
				idleTimer.Reset(c.maxConnectionIdle)
				continue
			}

			val := c.maxConnectionIdle - time.Since(idle)
			c.mu.Unlock()
			if val < 0 {
				c.close(SessionTimeoutErr)
				return
			}
			idleTimer.Reset(val)
		case <-c.closed.Fired():
			return
		case <-c.quit.Fired():
			return
		}

	}
}

type UserAgent struct {
	Protocal      string
	ClientId      string
	ClientVersion string
	OSType        string
}

type userAgentKey struct{}

func (c *qrpcConn) Serve() error {
	if c.quit.HasFired() {
		c.close(SessionTimeoutErr)
		return nil
	}

	defer func() {
		c.close(ServiceUnavailableErr)
	}()

	for {
		if c.quit.HasFired() {
			return nil
		}

		ctx := context.WithValue(c.ctx, userAgentKey{}, UserAgent{})
		stream, err := c.conn.AcceptStream(ctx)
		if err != nil {
			if appErr, ok := err.(*quic.ApplicationError); ok {
				if appErr.ErrorCode == quic.ApplicationErrorCode(quic.NoError) {
					return nil
				}
			}
			return err
		}

		msg, err := codec.DecodeOneMessage(stream, c.plmk)
		switch vv := msg.(type) {
		case *codec.Disconnect:
			return c.close(NoError)
		case *codec.Connect:
			c.idle = time.Now()
			fmt.Println("%s", vv.ClientId)
			return nil
		case *codec.Ping:
			c.idle = time.Now()
			pong := codec.PingAck{}
			if err := pong.Encode(stream); err != nil {
				return err
			}
		case *codec.Publish:
			c.idle = time.Now()
			c.handler(ctx, vv, stream)
		default:
			return c.close(UnSupportMessageErr)
		}
	}
}
