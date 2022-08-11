package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/aluka-7/metacode"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// 客户端日志
func (c *Client) clientLogging() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		startTime := time.Now()
		var peerInfo peer.Peer
		opts = append(opts, grpc.Peer(&peerInfo))

		// 调用者请求
		err := invoker(ctx, method, req, reply, cc, opts...)

		// 请求完成后
		cause := metacode.Cause(err)
		dt := time.Since(startTime)
		// 监控
		metricClientReqDur.Observe(int64(dt/time.Millisecond), method)
		metricClientReqCodeTotal.Inc(method, strconv.Itoa(cause.Code()))
		// 组装客户端日志
		if c.conf.EnableLog {
			var stack string
			if err != nil {
				stack = fmt.Sprintf("%+v", err)
			}
			cl := newClientLogging(method, peerInfo.Addr.String(), req.(fmt.Stringer).String(), reply.(fmt.Stringer).String(), stack, cause.Code(), dt.Seconds())
			v, _ := json.Marshal(cl)
			log.Info().Msg(string(v))
		}
		return err
	}
}

// 服务器日志记录
func (s *Server) serverLogging() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()
		caller := metacode.ToString(ctx, metacode.Caller)
		if caller == "" {
			caller = "no_user"
		}
		var ip string
		if peerInfo, ok := peer.FromContext(ctx); ok {
			ip = peerInfo.Addr.String()
		}
		var quota float64
		if deadline, ok := ctx.Deadline(); ok {
			quota = time.Until(deadline).Seconds()
		}

		// 调用服务器处理程序
		resp, err := handler(ctx, req)

		// 服务器响应后
		cause := metacode.Cause(err)
		dt := time.Since(startTime)

		// 监控
		metricServerReqDur.Observe(int64(dt/time.Millisecond), info.FullMethod, caller)
		metricServerReqCodeTotal.Inc(info.FullMethod, caller, strconv.Itoa(cause.Code()))

		if s.conf.EnableLog && dt > 500*time.Millisecond {
			var stack string
			if err != nil {
				stack = fmt.Sprintf("%+v", err)
			}
			sl := newServiceLogging(caller, ip, info.FullMethod, req.(fmt.Stringer).String(), stack, cause.Code(), dt.Seconds(), quota)
			v, _ := json.Marshal(sl)
			log.Info().Msg(string(v))
		}
		return resp, err
	}
}

type serviceLogging struct {
	User         string  `json:"user"`
	Ip           string  `json:"ip"`
	Path         string  `json:"path"`
	Ret          int     `json:"ret"`
	Ts           float64 `json:"ts"`
	TimeoutQuota float64 `json:"timeoutQuota"`
	Req          string  `json:"req"`
	Stack        string  `json:"stack,omitempty"`
}

func newServiceLogging(user, ip, path, req, stack string, ret int, ts, timeoutQuota float64) serviceLogging {
	return serviceLogging{user, ip, path, ret, ts, timeoutQuota, req, stack}
}

type clientLogging struct {
	Path  string  `json:"path"`
	Ip    string  `json:"ip"`
	Ret   int     `json:"ret"`
	Ts    float64 `json:"ts"`
	Args  string  `json:"args"`
	Reply string  `json:"reply"`
	Stack string  `json:"stack,omitempty"`
}

func newClientLogging(path, ip, args, reply, stack string, ret int, ts float64) clientLogging {
	return clientLogging{path, ip, ret, ts, args, reply, stack}
}
