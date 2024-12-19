package qrpc

import (
	"reflect"
	"testing"

	"github.com/stonefire-oss/stonefire-im/pkg/codec"
	"google.golang.org/grpc/mem"
)

func Test_decompressPayload(t *testing.T) {
	type args struct {
		rp codec.Payload
	}
	rp := []byte("0123456789")
	bs := mem.BufferSlice{mem.NewBuffer(&rp, nil)}
	bf, err := compressPayload(bs)
	defer bf.Free()
	if err != nil {
		panic(err)
	}
	tests := []struct {
		name    string
		args    args
		want    mem.BufferSlice
		wantErr bool
	}{
		{
			name:    "test",
			args:    args{rp: bf},
			want:    bs,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decompressPayload(tt.args.rp)
			if (err != nil) != tt.wantErr {
				t.Errorf("decompressPayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decompressPayload() = %v, want %v", got, tt.want)
			}
			got.Free()
		})
	}
}
