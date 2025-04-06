package storeandforward

import (
	"context"
	"errors"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-msgio"
	"github.com/mobazha/mobazha3.0/libs/store-and-forward/pb"
)

var ReadWriteTimeout = time.Second * 30

func writeMsgWithTimeout(w msgio.Writer, pmes *pb.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), ReadWriteTimeout)
	defer cancel()

	errCh := make(chan error)
	go func() {
		msgBytes, err := proto.Marshal(pmes)
		if err != nil {
			errCh <- err
			return
		}
		errCh <- w.WriteMsg(msgBytes)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return errors.New("write message timeout")
	}
}

func readMsgWithTimeout(r msgio.Reader, msg proto.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), ReadWriteTimeout)
	defer cancel()

	doneCh := make(chan error)
	go func() {
		msgBytes, err := r.ReadMsg()
		if err != nil {
			doneCh <- err
			return
		}
		err = proto.Unmarshal(msgBytes, msg)
		doneCh <- err
	}()

	select {
	case err := <-doneCh:
		return err
	case <-ctx.Done():
		return errors.New("read message timeout")
	}
}
