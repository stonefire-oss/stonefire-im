package qrpc

// Grpc status code [gRPC documentation]: https://github.com/grpc/grpc/blob/master/doc/statuscodes.md
type Code uint8

const (
	OK Code = 0

	Canceled Code = 1

	Unknown Code = 2

	InvalidArgument Code = 3

	DeadlineExceeded Code = 4

	NotFound Code = 5

	AlreadyExists Code = 6

	PermissionDenied Code = 7

	ResourceExhausted Code = 8

	FailedPrecondition Code = 9

	Aborted Code = 10

	OutOfRange Code = 11

	Unimplemented Code = 12

	Internal Code = 13

	Unavailable Code = 14

	DataLoss Code = 15

	Unauthenticated Code = 16

	_maxCode = 17
)

type Status struct {
	Code    Code
	Message string
}
