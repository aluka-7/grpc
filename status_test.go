package grpc

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aluka-7/metacode"
	"github.com/golang/protobuf/ptypes/timestamp"
	pkgerr "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCodeConvert(t *testing.T) {
	var table = map[codes.Code]metacode.Code{
		codes.OK: metacode.OK,
		// codes.Canceled
		codes.Unknown:          metacode.ServerErr,
		codes.InvalidArgument:  metacode.RequestErr,
		codes.DeadlineExceeded: metacode.Deadline,
		codes.NotFound:         metacode.NothingFound,
		// codes.AlreadyExists
		codes.PermissionDenied:  metacode.AccessDenied,
		codes.ResourceExhausted: metacode.LimitExceed,
		// codes.FailedPrecondition
		// codes.Aborted
		// codes.OutOfRange
		codes.Unimplemented: metacode.MethodNotAllowed,
		codes.Unavailable:   metacode.ServiceUnavailable,
		// codes.DataLoss
		codes.Unauthenticated: metacode.Unauthorized,
	}
	for k, v := range table {
		assert.Equal(t, toMetaCode(status.New(k, "-500")), v)
	}
	for k, v := range table {
		assert.Equal(t, togRrcCode(v), k, fmt.Sprintf("togRPC code error: %d -> %d", v, k))
	}
}

func TestNoDetailsConvert(t *testing.T) {
	gst := status.New(codes.Unknown, "-2233")
	assert.Equal(t, toMetaCode(gst).Code(), -2233)

	gst = status.New(codes.Internal, "")
	assert.Equal(t, toMetaCode(gst).Code(), -500)
}

func TestFromError(t *testing.T) {
	t.Run("input general error", func(t *testing.T) {
		err := errors.New("general error")
		gst := FromError(err)

		assert.Equal(t, codes.Unknown, gst.Code())
		assert.Contains(t, gst.Message(), "general")
	})
	t.Run("input wrap error", func(t *testing.T) {
		err := pkgerr.Wrap(metacode.RequestErr, "hh")
		gst := FromError(err)

		assert.Equal(t, "-400", gst.Message())
	})
	t.Run("input metacode.Code", func(t *testing.T) {
		err := metacode.RequestErr
		gst := FromError(err)

		// assert.Equal(t, codes.InvalidArgument, gst.Code())
		// NOTE: set all grpc.status as Unknown when error is metacode.Codes for compatible
		assert.Equal(t, codes.Unknown, gst.Code())
		// NOTE: gst.Message == str(metacode.Code) for compatible php leagcy code
		assert.Equal(t, err.Message(), gst.Message())
	})
	t.Run("input raw Canceled", func(t *testing.T) {
		gst := FromError(context.Canceled)

		assert.Equal(t, codes.Unknown, gst.Code())
		assert.Equal(t, "-498", gst.Message())
	})
	t.Run("input raw DeadlineExceeded", func(t *testing.T) {
		gst := FromError(context.DeadlineExceeded)

		assert.Equal(t, codes.Unknown, gst.Code())
		assert.Equal(t, "-504", gst.Message())
	})
	t.Run("input metacode.Status", func(t *testing.T) {
		m := &timestamp.Timestamp{Seconds: time.Now().Unix()}
		err, _ := metacode.Error(metacode.Unauthorized, "unauthorized").WithDetails(m)
		gst := FromError(err)

		// NOTE: set all grpc.status as Unknown when error is metacode.Codes for compatible
		assert.Equal(t, codes.Unknown, gst.Code())
		assert.Len(t, gst.Details(), 1)
		details := gst.Details()
		assert.IsType(t, err.Proto(), details[0])
	})
}

func TestToMetaCode(t *testing.T) {
	t.Run("input general grpc.Status", func(t *testing.T) {
		gst := status.New(codes.Unknown, "unknown")
		ec := ToMetaCode(gst)

		assert.Equal(t, int(metacode.ServerErr), ec.Code())
		assert.Equal(t, "-500", ec.Message())
		assert.Len(t, ec.Details(), 0)
	})
	t.Run("input metacode.Status", func(t *testing.T) {
		m := &timestamp.Timestamp{Seconds: time.Now().Unix()}
		st, _ := metacode.Errorf(metacode.Unauthorized, "Unauthorized").WithDetails(m)
		gst := status.New(codes.InvalidArgument, "requesterr")
		gst, _ = gst.WithDetails(st.Proto())
		ec := ToMetaCode(gst)

		assert.Equal(t, int(metacode.Unauthorized), ec.Code())
		assert.Equal(t, "Unauthorized", ec.Message())
		assert.Len(t, ec.Details(), 1)
		assert.IsType(t, m, ec.Details()[0])
	})
}
