package grpc

import (
	"context"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/aluka-7/metacode"
)

// 将metacode.Codes转换为gRPC代码
func togRrcCode(code metacode.Codes) codes.Code {
	switch code.Code() {
	case metacode.OK.Code():
		return codes.OK
	case metacode.RequestErr.Code():
		return codes.InvalidArgument
	case metacode.NothingFound.Code():
		return codes.NotFound
	case metacode.Unauthorized.Code():
		return codes.Unauthenticated
	case metacode.AccessDenied.Code():
		return codes.PermissionDenied
	case metacode.LimitExceed.Code():
		return codes.ResourceExhausted
	case metacode.MethodNotAllowed.Code():
		return codes.Unimplemented
	case metacode.Deadline.Code():
		return codes.DeadlineExceeded
	case metacode.ServiceUnavailable.Code():
		return codes.Unavailable
	}
	return codes.Unknown
}

func toMetaCode(gst *status.Status) metacode.Code {
	code := gst.Code()
	switch code {
	case codes.OK:
		return metacode.OK
	case codes.InvalidArgument:
		return metacode.RequestErr
	case codes.NotFound:
		return metacode.NothingFound
	case codes.PermissionDenied:
		return metacode.AccessDenied
	case codes.Unauthenticated:
		return metacode.Unauthorized
	case codes.ResourceExhausted:
		return metacode.LimitExceed
	case codes.Unimplemented:
		return metacode.MethodNotAllowed
	case codes.DeadlineExceeded:
		return metacode.Deadline
	case codes.Unavailable:
		return metacode.ServiceUnavailable
	case codes.Unknown:
		return metacode.String(gst.Message())
	}
	return metacode.ServerErr
}

// 转换服务回复错误并尝试将其转换为grpc.Status.
func FromError(svrErr error) (gst *status.Status) {
	var err error
	svrErr = errors.Cause(svrErr)
	if code, ok := svrErr.(metacode.Codes); ok {
		// 处理错误
		if gst, err = gRPCStatusFromMetaCode(code); err == nil {
			return
		}
	}
	// 对于某些特殊错误,请转换context.Canceled为metacode.Canceled,
	// context.DeadlineExceeded到metacode.DeadlineExceeded仅用于原始错误(如果将err包装)将不起作用.
	switch svrErr {
	case context.Canceled:
		gst, _ = gRPCStatusFromMetaCode(metacode.Canceled)
	case context.DeadlineExceeded:
		gst, _ = gRPCStatusFromMetaCode(metacode.Deadline)
	default:
		gst, _ = status.FromError(svrErr)
	}
	return
}

func gRPCStatusFromMetaCode(code metacode.Codes) (*status.Status, error) {
	var st *metacode.Status
	switch v := code.(type) {
	case *metacode.Status:
		st = v
	case metacode.Code:
		st = metacode.FromCode(v)
	default:
		st = metacode.Error(metacode.Code(code.Code()), code.Message())
		for _, detail := range code.Details() {
			if msg, ok := detail.(proto.Message); ok {
				st.WithDetails(msg)
			}
		}
	}
	gst := status.New(codes.Unknown, strconv.Itoa(st.Code()))
	return gst.WithDetails(st.Proto())
}

// 将grpc.status转换为metacode.Codes
func ToMetaCode(gst *status.Status) metacode.Codes {
	details := gst.Details()
	for _, detail := range details {
		// 将详细信息转换为状态,仅使用第一个详细信息
		if pb, ok := detail.(proto.Message); ok {
			return metacode.FromProto(pb)
		}
	}
	return toMetaCode(gst)
}
