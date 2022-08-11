package grpc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aluka-7/configuration"
	"github.com/aluka-7/trace"
	"google.golang.org/grpc"
)

type RpcServerConfig struct {
	*ServerConfig
	Tag []trace.Tag `json:"tag"`
}
type RpcClientConfig struct {
	*ClientConfig
	Target string `json:"target"`
}

type RpcEngine interface {
	Server(monitor bool, app, group, path string, handlers ...grpc.UnaryServerInterceptor) (*Server, *RpcServerConfig)
	ClientConn(systemId string, handlers ...grpc.UnaryClientInterceptor) (conn *grpc.ClientConn, cc *RpcClientConfig)
}

func Engine(systemId string, cfg configuration.Configuration) RpcEngine {
	fmt.Println("Loading FoChange Rpc Engine ver:1.1.0")
	return &rpcEngine{cfg: cfg, systemId: systemId}
}

type rpcEngine struct {
	systemId string
	cfg      configuration.Configuration
}

func (r *rpcEngine) Server(monitor bool, app, group, path string, handlers ...grpc.UnaryServerInterceptor) (*Server, *RpcServerConfig) {
	scc := &serverConfigChanged{path: fmt.Sprintf("/system/%s/%s/%s", app, group, path), handlers: handlers}
	r.cfg.Get(app, group, "", []string{path}, scc)
	if monitor {
		go metrics()
	}
	return scc.Server()
}

type serverConfigChanged struct {
	path     string
	handlers []grpc.UnaryServerInterceptor
	server   *Server
	cfg      *RpcServerConfig
}

func (scc *serverConfigChanged) Changed(data map[string]string) {
	if v, ok := data[scc.path]; ok {
		scc.cfg = new(RpcServerConfig)
		if err := json.Unmarshal([]byte(v), scc.cfg); err == nil {
			if scc.server == nil {
				scc.server = NewServer(scc.cfg.ServerConfig)
				if len(scc.handlers) > 0 {
					scc.server.Use(scc.handlers...)
				}
			} else {
				fmt.Printf("更新[%s]RPC服务器配置:%s\n", scc.path, v)
				_ = scc.server.SetConfig(scc.cfg.ServerConfig)
			}
		} else {
			panic(fmt.Sprintf("从配置中心读取[%s]RPC服务器配置出错:%+v", scc.path, err))
		}

	} else {
		panic(fmt.Sprintf("配置中心不存在[%s]RPC服务器配置", scc.path))
	}
}
func (scc *serverConfigChanged) Server() (*Server, *RpcServerConfig) {
	return scc.server, scc.cfg
}
func (r *rpcEngine) ClientConn(systemId string, handlers ...grpc.UnaryClientInterceptor) (*grpc.ClientConn, *RpcClientConfig) {
	ccc := &clientConfigChanged{path: fmt.Sprintf("/system/base/rpc/%s", systemId), handlers: handlers}
	r.cfg.Get("base", "rpc", "", []string{systemId}, ccc)
	client, cfg := ccc.Client()
	conn, err := client.Dial(context.Background(), cfg.Target, []string{r.systemId})
	if err != nil {
		panic(fmt.Sprintf("RPC连接远程服务出错:%+v\n", err))
	}
	return conn, cfg
}

type clientConfigChanged struct {
	path     string
	handlers []grpc.UnaryClientInterceptor
	client   *Client
	cfg      *RpcClientConfig
}

func (ccc *clientConfigChanged) Changed(data map[string]string) {
	if v, ok := data[ccc.path]; ok {
		ccc.cfg = new(RpcClientConfig)
		if err := json.Unmarshal([]byte(v), ccc.cfg); err == nil {
			if ccc.cfg.Timeout == 0 || ccc.cfg.KeepAliveInterval == 0 || ccc.cfg.KeepAliveTimeout == 0 {
				if ccc.client == nil {
					panic("Timeout,KeepAliveInterval以及KeepAliveTimeout,必须大于零")
				} else {
					fmt.Println("Timeout,KeepAliveInterval以及KeepAliveTimeout,必须大于零,此设置没有生效")
					return
				}
			}
			if ccc.client == nil {
				ccc.client = NewClient(ccc.cfg.ClientConfig)
				if len(ccc.handlers) > 0 {
					ccc.client.Use(ccc.handlers...)
				}
			} else {
				fmt.Printf("更新[%s]RPC客户端配置为:%s\n", ccc.path, v)
				_ = ccc.client.SetConfig(ccc.cfg.ClientConfig)
			}
		} else {
			panic(fmt.Sprintf("从配置中心读取[%s]RPC客户端配置出错:%+v", ccc.path, err))
		}

	} else {
		panic(fmt.Sprintf("配置中心不存在[%s]RPC客户端配置", ccc.path))
	}
}
func (ccc *clientConfigChanged) Client() (*Client, *RpcClientConfig) {
	return ccc.client, ccc.cfg
}
