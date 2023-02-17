package grpc

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/aluka-7/metacode"
	"github.com/aluka-7/trace"
	"github.com/aluka-7/utils"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// ClientConfig rpc客户端配置.
type ClientConfig struct {
	Dial                utils.Duration           `json:"dial"`
	Timeout             utils.Duration           `json:"timeout"`
	Method              map[string]*ClientConfig `json:"method"`
	NonBlock            bool                     `json:"nonBlock"`
	KeepAliveInterval   utils.Duration           `json:"keepAliveInterval"`
	KeepAliveTimeout    utils.Duration           `json:"keepAliveTimeout"`
	PermitWithoutStream bool                     `json:"permitWithoutStream"`
	EnableLog           bool                     `json:"enableLog"`
}

// Client 客户端是框架的客户端实例,它包含ctx,opt和拦截器。
// 使用NewClient()创建Client的实例.
type Client struct {
	conf  *ClientConfig
	mutex sync.RWMutex

	opts     []grpc.DialOption
	handlers []grpc.UnaryClientInterceptor
}

// TimeoutCallOption 超时选项.
type TimeoutCallOption struct {
	*grpc.EmptyCallOption
	Timeout time.Duration
}

// WithTimeoutCallOption 可以覆盖ctx中的超时和配置文件中的超时
func WithTimeoutCallOption(timeout time.Duration) *TimeoutCallOption {
	return &TimeoutCallOption{&grpc.EmptyCallOption{}, timeout}
}

// 为OpenTracing\Logging\LinkTimeout返回一个新的一元客户端拦截器.
func (c *Client) handle(caller []string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
		var ok bool
		var t trace.Trace
		var gmd metadata.MD
		var conf *ClientConfig
		var cancel context.CancelFunc
		var addr string
		var p peer.Peer
		var ec metacode.Codes = metacode.OK
		// apm tracing
		if t, ok = trace.FromContext(ctx); ok {
			t = t.Fork("", method)
			defer t.Finish(&err)
		}

		// 设置元数据
		gmd = metadata.MD{metacode.Caller: caller}
		_ = trace.Inject(t, trace.GRPCFormat, gmd)
		c.mutex.RLock()
		if conf, ok = c.conf.Method[method]; !ok {
			conf = c.conf
		}
		c.mutex.RUnlock()

		var timeOpt *TimeoutCallOption
		for _, opt := range opts {
			var tok bool
			timeOpt, tok = opt.(*TimeoutCallOption)
			if tok {
				break
			}
		}
		if timeOpt != nil && timeOpt.Timeout > 0 {
			ctx, cancel = context.WithTimeout(metacode.WithContext(ctx), timeOpt.Timeout)
		} else {
			_, ctx, cancel = conf.Timeout.Shrink(ctx)
		}

		defer cancel()
		metacode.Range(ctx, func(key string, value interface{}) {
			if v, ok := value.(string); ok {
				gmd[key] = []string{v}
			}
		}, metacode.IsOutgoingKey)
		// merge with old metadata if exists
		if old, ok := metadata.FromOutgoingContext(ctx); ok {
			gmd = metadata.Join(gmd, old)
		}
		ctx = metadata.NewOutgoingContext(ctx, gmd)

		opts = append(opts, grpc.Peer(&p))
		if err = invoker(ctx, method, req, reply, cc, opts...); err != nil {
			gst, _ := status.FromError(err)
			ec = ToMetaCode(gst)
			err = errors.WithMessage(ec, gst.Message())
		}
		if p.Addr != nil {
			addr = p.Addr.String()
		}
		if t != nil {
			t.SetTag(trace.String(trace.TagAddress, addr), trace.String(trace.TagComment, ""))
		}
		return
	}
}

// NewConn 创建rpc连接.
func NewConn(target string, conf *ClientConfig, caller []string, opt ...grpc.DialOption) (*grpc.ClientConn, error) {
	return NewClient(conf, opt...).Dial(context.Background(), target, caller, opt...)
}

// NewClient 返回带有默认客户端拦截器的新的空白Client实例.
// opt可用于添加rpc拨号选项.
func NewClient(conf *ClientConfig, opt ...grpc.DialOption) *Client {
	c := new(Client)
	if err := c.SetConfig(conf); err != nil {
		panic(err)
	}
	c.UseOpt(opt...)
	return c
}

// SetConfig 热重载客户端配置
func (c *Client) SetConfig(conf *ClientConfig) (err error) {
	c.mutex.Lock()
	c.conf = conf
	c.mutex.Unlock()
	return nil
}

// Use Use将全局拦截器附加到客户端。
// 例如:这是断路器或错误管理拦截器的正确位置。
func (c *Client) Use(handlers ...grpc.UnaryClientInterceptor) *Client {
	finalSize := len(c.handlers) + len(handlers)
	if finalSize >= int(_abortIndex) {
		panic("rpc: client use too many handlers")
	}
	mergedHandlers := make([]grpc.UnaryClientInterceptor, finalSize)
	copy(mergedHandlers, c.handlers)
	copy(mergedHandlers[len(c.handlers):], handlers)
	c.handlers = mergedHandlers
	return c
}

// UseOpt 将全局 rpc DialOption 附加到客户端.
func (c *Client) UseOpt(opts ...grpc.DialOption) *Client {
	c.opts = append(c.opts, opts...)
	return c
}

func (c *Client) cloneOpts() []grpc.DialOption {
	dialOptions := make([]grpc.DialOption, len(c.opts))
	copy(dialOptions, c.opts)
	return dialOptions
}

func (c *Client) dial(ctx context.Context, target string, caller []string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	// 克隆连接配置
	dialOptions := c.cloneOpts()
	if !c.conf.NonBlock {
		dialOptions = append(dialOptions, grpc.WithBlock())
	}
	dialOptions = append(dialOptions, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                time.Duration(c.conf.KeepAliveInterval),
		Timeout:             time.Duration(c.conf.KeepAliveTimeout),
		PermitWithoutStream: !c.conf.PermitWithoutStream,
	}))
	dialOptions = append(dialOptions, opts...)

	// 初始化默认处理程序
	var handlers []grpc.UnaryClientInterceptor
	handlers = append(handlers, c.recovery())
	handlers = append(handlers, c.clientLogging())
	handlers = append(handlers, c.handlers...)
	// 注意:c.handle必须是最后一个拦截器.
	handlers = append(handlers, c.handle(caller))

	dialOptions = append(dialOptions, grpc.WithUnaryInterceptor(chainUnaryClient(handlers)))
	c.mutex.RLock()
	conf := c.conf
	c.mutex.RUnlock()
	if conf.Dial > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(conf.Dial))
		defer cancel()
	}
	if conn, err = grpc.DialContext(ctx, target, dialOptions...); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "rpc client: dial %s error %v!", target, err)
	}
	err = errors.WithStack(err)
	return
}

func (c *Client) Dial(ctx context.Context, target string, caller []string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	opts = append(opts, grpc.WithInsecure())
	return c.dial(ctx, target, caller, opts...)
}

// DialTLS 通过tls传输创建到给定目标的客户端连接.
func (c *Client) DialTLS(ctx context.Context, target string, file string, name string, caller []string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	var crt credentials.TransportCredentials
	crt, err = credentials.NewClientTLSFromFile(file, name)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	opts = append(opts, grpc.WithTransportCredentials(crt))
	return c.dial(ctx, target, caller, opts...)
}

// 返回从任何紧急情况中恢复的客户端拦截器.
func (c *Client) recovery() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
		defer func() {
			if rerr := recover(); rerr != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				rs := runtime.Stack(buf, false)
				if rs > size {
					rs = size
				}
				buf = buf[:rs]
				_, _ = fmt.Fprintf(os.Stderr, fmt.Sprintf("grpc client panic: %v\n%v\n%v\n%s\n", req, reply, rerr, buf))
				err = metacode.ServerErr
			}
		}()
		err = invoker(ctx, method, req, reply, cc, opts...)
		return
	}
}

// 从许多拦截器链中创建一个拦截器.
//
// 执行以从左到右的顺序进行,包括传递上下文。
// 例如:ChainUnaryClient(1、2、3)将在三之前的两个之前执行一个
func chainUnaryClient(handlers []grpc.UnaryClientInterceptor) grpc.UnaryClientInterceptor {
	n := len(handlers)
	if n == 0 {
		return func(ctx context.Context, method string, req, reply interface{},
			cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
	}

	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		var (
			i            int
			chainHandler grpc.UnaryInvoker
		)
		chainHandler = func(ictx context.Context, imethod string, ireq, ireply interface{}, ic *grpc.ClientConn, iopts ...grpc.CallOption) error {
			if i == n-1 {
				return invoker(ictx, imethod, ireq, ireply, ic, iopts...)
			}
			i++
			return handlers[i](ictx, imethod, ireq, ireply, ic, chainHandler, iopts...)
		}

		return handlers[0](ctx, method, req, reply, cc, chainHandler, opts...)
	}
}
