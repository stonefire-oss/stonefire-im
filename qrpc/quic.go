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
	ApplicationErr        = 0xFFFF

	SessionTimeoutErrMsg     = "session timeout"
	UnSupportMessageErrMsg   = "unsupport message type"
	ServiceUnavailableErrMsg = "service unavailable"
)

type CloseReason uint64

func (r CloseReason) code() quic.ApplicationErrorCode {
	return quic.ApplicationErrorCode(r)
}

func (r CloseReason) String() string {
	switch r {
	case NoError:
		return "no error"
	case SessionTimeoutErr:
		return SessionTimeoutErrMsg
	case UnSupportMessageErr:
		return UnSupportMessageErrMsg
	case ServiceUnavailableErr:
		return ServiceUnavailableErrMsg
	default:
		return fmt.Sprintf("unknown code %d", r)
	}
}

type qrpcConn struct {
	conn              quic.Connection
	quit              *utils.Event
	closed            *utils.Event
	mu                sync.Mutex
	idle              time.Time
	maxConnectionIdle time.Duration
	ctx               context.Context
	plmk              codec.PayloadBuilder
}

func newQRPConn(conn quic.Connection, s *Server) *qrpcConn {
	qc := &qrpcConn{
		conn:              conn,
		quit:              s.quit,
		closed:            utils.NewEvent(),
		maxConnectionIdle: s.opts.maxConnectionIdle,
		ctx:               context.Background(),
		plmk:              &pooledPLMaker{s.opts.bufferPool},
	}
	return qc
}

func (c *qrpcConn) closeWithReason(r CloseReason) error {
	return c.close(r.code(), r.String())
}

func (c *qrpcConn) close(code quic.ApplicationErrorCode, msg string) error {
	c.mu.Lock()

	defer func() {
		c.mu.Unlock()
	}()

	if c.closed.Fire() {
		return c.conn.CloseWithError(code, msg)
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
				c.closeWithReason(SessionTimeoutErr)
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

type serverConnKey struct{}

func qrpcConnFromContext(ctx context.Context) *qrpcConn {
	if ctx == nil {
		return nil
	}

	if qrpc, ok := ctx.Value(serverConnKey{}).(*qrpcConn); ok {
		return qrpc
	}
	return nil
}

func (c *qrpcConn) Serve(handler streamHandler) error {
	if c.quit.HasFired() {
		c.closeWithReason(ServiceUnavailableErr)
		return nil
	}

	go c.keepalive()

	defer func() {
		c.closeWithReason(SessionTimeoutErr)
	}()

	for {
		if c.quit.HasFired() {
			return nil
		}

		ctx := context.WithValue(c.ctx, serverConnKey{}, c)
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
		if err != nil {
			return err
		}
		switch vv := msg.(type) {
		case *codec.Disconnect:
			return c.closeWithReason(NoError)
		case *codec.Connect:
			c.idle = time.Now()
			return nil
		case *codec.Ping:
			c.idle = time.Now()
			pong := codec.PingAck{}
			if err := pong.Encode(stream); err != nil {
				return err
			}
		case *codec.Publish:
			c.idle = time.Now()
			handler(ctx, vv, stream)
		default:
			if pc, ok := vv.(codec.PayloadContainer); ok {
				FreePayload(pc)
			}
			return c.closeWithReason(UnSupportMessageErr)
		}
	}
}
