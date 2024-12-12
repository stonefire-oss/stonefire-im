package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"

	"github.com/quic-go/quic-go"
	"github.com/stonefire-oss/stonefire-im/demo/pb"
	"github.com/stonefire-oss/stonefire-im/qrpc"
	"google.golang.org/grpc"
)

type studentSrv struct {
	pb.UnimplementedStudentServiceServer
}

func (s *studentSrv) CreateStudent(ctx context.Context, st *pb.Student) (*pb.Result, error) {
	return &pb.Result{Code: 10, Message: st.Name}, nil
}

func (s *studentSrv) Hello(stream grpc.BidiStreamingServer[pb.Student, pb.Echo]) error {
	return nil
}

const addr = "localhost:4242"

const message = "foobar"

func main() {
	s := qrpc.NewServer()
	pb.RegisterStudentServiceServer(s, &studentSrv{})
	listener, err := quic.ListenAddr(addr, generateTLSConfig(), nil)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	s.Serve(listener)
}

func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-echo-example"},
	}
}
