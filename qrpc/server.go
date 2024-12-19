package qrpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/stonefire-oss/stonefire-im/pkg/codec"
	"github.com/stonefire-oss/stonefire-im/pkg/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/mem"
)

const (
	defaultServerMaxReceiveMessageSize = 1024 * 1024 * 4
	defaultServerMaxSendMessageSize    = math.MaxInt32
	defaultMaxConcurrentStreams        = 100
	defaultMaxConnectionIdle           = time.Second * 3
)

type ServerOption interface {
	apply(*serverOptions)
}

type serverOptions struct {
	maxReceiveMessageSize int
	maxSendMessageSize    int
	numServerWorkers      uint32
	maxConnectionIdle     time.Duration
	maxConcurrentStreams  uint32

	bufferPool mem.BufferPool
}

type streamHandler func(ctx context.Context, req *codec.Publish, stream quic.Stream)

// serviceInfo wraps information about a service. It is very similar to
// ServiceDesc and is constructed from it for internal purposes.
type serviceInfo struct {
	// Contains the implementation for the methods in this service.
	serviceImpl any
	methods     map[string]*grpc.MethodDesc
	streams     map[string]*grpc.StreamDesc
	mdata       any
}

type Server struct {
	opts serverOptions
	mu   sync.Mutex // guards following

	cv *sync.Cond

	serveWG    sync.WaitGroup
	handlersWG sync.WaitGroup

	quit *utils.Event
	done *utils.Event

	ctx       context.Context
	cancelFun context.CancelFunc

	services map[string]*serviceInfo

	serverWorkerChannel      chan func()
	serverWorkerChannelClose func()
}

const serverWorkerResetThreshold = 1 << 16

func (s *Server) serverWorker() {
	for completed := 0; completed < serverWorkerResetThreshold; completed++ {
		f, ok := <-s.serverWorkerChannel
		if !ok {
			return
		}
		f()
	}
	go s.serverWorker()
}

func (s *Server) initServerWorkers() {
	s.serverWorkerChannel = make(chan func())
	s.serverWorkerChannelClose = sync.OnceFunc(func() {
		close(s.serverWorkerChannel)
	})
	for i := uint32(0); i < s.opts.numServerWorkers; i++ {
		go s.serverWorker()
	}
}

var defaultServerOptions = serverOptions{
	maxReceiveMessageSize: defaultServerMaxReceiveMessageSize,
	maxSendMessageSize:    defaultServerMaxSendMessageSize,
	maxConcurrentStreams:  defaultMaxConcurrentStreams,
	maxConnectionIdle:     defaultMaxConnectionIdle,
	bufferPool:            mem.DefaultBufferPool(),
}

func NewServer(opt ...ServerOption) *Server {
	opts := defaultServerOptions
	for _, o := range opt {
		o.apply(&opts)
	}
	s := &Server{
		opts: opts,
		quit: utils.NewEvent(),
		done: utils.NewEvent(),

		services: make(map[string]*serviceInfo),
	}
	s.cv = sync.NewCond(&s.mu)
	if s.opts.numServerWorkers > 0 {
		s.initServerWorkers()
	}
	s.ctx, s.cancelFun = context.WithCancel(context.Background())
	return s
}

func (s *Server) RegisterService(sd *grpc.ServiceDesc, ss any) {
	if ss != nil {
		ht := reflect.TypeOf(sd.HandlerType).Elem()
		st := reflect.TypeOf(ss)
		if !st.Implements(ht) {
			//logger.Fatalf("grpc: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
			err := fmt.Errorf("grpc: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
			panic(err)
		}
	}
	s.register(sd, ss)
}

func (s *Server) register(sd *grpc.ServiceDesc, ss any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info := &serviceInfo{
		serviceImpl: ss,
		methods:     make(map[string]*grpc.MethodDesc),
		streams:     make(map[string]*grpc.StreamDesc),
		mdata:       sd.Metadata,
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		info.methods[d.MethodName] = d
	}
	for i := range sd.Streams {
		d := &sd.Streams[i]
		info.streams[d.StreamName] = d
	}
	s.services[sd.ServiceName] = info
}

func (s *Server) Serve(ls *quic.Listener) error {
	defer func() {
		ls.Close()
	}()
	for {
		conn, err := ls.Accept(s.ctx)
		var tempDelay time.Duration

		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			if ne, ok := err.(interface {
				Temporary() bool
			}); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				timer := time.NewTimer(tempDelay)
				select {
				case <-timer.C:
				case <-s.quit.Fired():
					timer.Stop()
					return nil
				}
				continue
			}
			return err
		}

		s.handleRawConn(conn)
	}
}

func (s *Server) stop() {
	s.quit.Fire()
	s.cancelFun()
}

func (s *Server) handleStream(ctx context.Context, req *codec.Publish, stream quic.Stream) {
	sm := req.Path
	if sm != "" && sm[0] == '/' {
		sm = sm[1:]
	}
	pos := strings.LastIndex(sm, "/")
	service := sm[:pos]
	method := sm[pos+1:]
	srv, knownService := s.services[service]
	if knownService {
		if md, ok := srv.methods[method]; ok {
			s.processUnaryRPC(ctx, md, srv, req, stream)
			stream.Close()
			return
		}
		if sd, ok := srv.streams[method]; ok {
			s.processStreamingRPC(ctx, sd, srv, req, stream)
			return
		}
	}

	ack := codec.PubAck{
		Header:    codec.Header{AckRequired: req.AckRequired},
		MessageId: req.MessageId,
		Status:    codec.Status{Code: uint8(Unimplemented)},
	}

	ack.Encode(stream)
	stream.Close()
}

func (s *Server) processStreamingRPC(ctx context.Context, sd *grpc.StreamDesc, info *serviceInfo, req *codec.Publish, stream quic.Stream) error {
	ss := newServerStream(ctx, req, stream, qrpcConnFromContext(ctx), sd)
	return sd.Handler(info.serviceImpl, ss)
}

func (s *Server) processUnaryRPC(ctx context.Context, md *grpc.MethodDesc, info *serviceInfo, req *codec.Publish, stream quic.Stream) error {
	df := func(v any) error {
		defer FreePayload(req)
		return DecodePayload(v, req.Payload, req.Compressed)
	}
	reply, appErr := md.Handler(info.serviceImpl, ctx, df, nil)
	if appErr != nil {
		return appErr
	}

	bf, err := EncodePayload(reply, req.Compressed)

	if err != nil {
		return err
	}

	defer func() {
		bf.Free()
	}()

	ack := codec.PubAck{
		Header:    codec.Header{AckRequired: req.AckRequired, Compressed: req.Compressed},
		MessageId: req.MessageId,
		Payload:   bf,
	}

	return ack.Encode(stream)
}

func (s *Server) handleRawConn(conn quic.Connection) {
	s.serveWG.Add(1)
	streamQuota := utils.NewHandlerQuota(s.opts.maxConcurrentStreams)
	handler := func(ctx context.Context, req *codec.Publish, stream quic.Stream) {
		streamQuota.Acquire()

		f := func() {
			defer streamQuota.Release()
			s.handleStream(ctx, req, stream)
		}
		if s.opts.numServerWorkers > 0 {
			select {
			case s.serverWorkerChannel <- f:
				return
			default:
				ack := codec.PubAck{
					Header:    codec.Header{AckRequired: req.AckRequired},
					MessageId: req.MessageId,
					Status:    codec.Status{Code: uint8(ResourceExhausted)},
				}

				ack.Encode(stream)
				stream.Close()
				return
			}
		}
		go f()
	}

	qcon := newQRPConn(conn, s)
	s.serveWG.Done()
	f := func() {
		qcon.Serve(handler)
	}
	go f()
}
