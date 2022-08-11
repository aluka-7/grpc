# RPC简介
技术部的RPC框架,融合技术部的核心科技,带来如飞一般的体验。

# 相关配置信息说明

server配置项说明

```go
// 服务器配置信息
type ServerConfig struct {
    Network           string            `json:"network"`           // 网络为rpc监听网络，默认值为tcp
    Addr              string            `json:"address"`           // 地址是rpc监听地址，默认值为0.0.0.0:9000
    Timeout           utils.Duration    `json:"timeout"`           // 超时是每个rpc调用的上下文超时。
    IdleTimeout       utils.Duration    `json:"idleTimeout"`       // IdleTimeout是一段持续时间，在这段时间内可以通过发送GoAway关闭空闲连接。 空闲持续时间是自最近一次未完成RPC的数量变为零或建立连接以来定义的。
    MaxLifeTime       utils.Duration    `json:"maxLife"`           // MaxLifeTime是连接通过发送GoAway关闭之前可能存在的最长时间的持续时间。 将向+/- 10％的随机抖动添加到MaxConnectionAge中以分散连接风暴.
    ForceCloseWait    utils.Duration    `json:"closeWait"`         // ForceCloseWait是MaxLifeTime之后的附加时间，在此之后将强制关闭连接。
    KeepAliveInterval utils.Duration    `json:"keepaliveInterval"` // 如果服务器没有看到任何活动，则KeepAliveInterval将在此时间段之后，对客户端进行ping操作以查看传输是否仍然有效。
    KeepAliveTimeout  utils.Duration    `json:"keepaliveTimeout"`  // 进行keepalive检查ping之后，服务器将等待一段时间的超时，并且即使在关闭连接后也看不到活动。
    RateLimit         *ratelimit.Config `json:"limit"`             // 限流
    EnableLog         bool              `json:"enableLog"`         // 是否打开日记
}
```

对应zk中的信息:

地址为: /fc/base/app/9999

```json
'{"network":"tcp","address":"127.0.0.1:9090","timeout":"2s","idleTimeout":"2s","maxLife":"2s","closeWait":"2s","keepaliveInterval":"2s","keepaliveTimeout":"2s","limit":{"Enabled":false,"Window":0,"WinBucket":0,"Rule":"","Debug":false,"CPUThreshold":0},"enableLog":true}'
```

注意:`9999`为具体app中的systemId

## Client 配置项说明

```go
// ClientConfig是rpc客户端配置.
type ClientConfig struct {
    Dial                utils.Duration           `json:"dial"`
    Timeout             utils.Duration           `json:"timeout"`
    Breaker             *breaker.Config          `json:"breaker"`
    Method              map[string]*ClientConfig `json:"method"`
    NonBlock            bool                     `json:"nonBlock"`
    KeepAliveInterval   utils.Duration           `json:"keepAliveInterval"`
    KeepAliveTimeout    utils.Duration           `json:"keepAliveTimeout"`
    PermitWithoutStream bool                     `json:"permitWithoutStream"`
    EnableLog           bool                     `json:"enableLog"`
}
```

对应zk中的配置信息

### 客户端基础信息地址为: /fc/base/rpc/client/base

```json
{"dial":"10s","timeout":"10s","breaker":{"switchOff":false,"failureRate":0.8,"window":"1s"},"nonBlock":false,"keepAliveInterval":"10s","keepAliveTimeout":"10s","keepAliveWithoutStream":true,"enableLog":true}
```

### 客户端连接地址:/fc/base/rpc/client/999

```json
127.0.0.1:9090
```


# 背景

当代的互联网的服务，通常都是用复杂的、大规模分布式集群来实现的。互联网应用构建在不同的软件模块集上，这些软件模块，有可能是由不同的团队开发、可能使用不同的编程语言来实现、有可能布在了几千台服务器，横跨多个不同的数据中心。因此，就需要一些可以帮助理解系统行为、用于分析性能问题的工具。

# 概览

* trace基于opentracing语义
* 全链路支持（gRPC/HTTP/MySQL/Redis/Memcached等）
 
## 参考文档

[opentracing](https://github.com/opentracing-contrib/opentracing-specification-zh/blob/master/specification.md)  
[dapper](https://bigbully.github.io/Dapper-translation/)

# 使用

本身不提供整套`trace`数据方案，但在`git.forchange.cn/framework/trace/report.go`内声明了`repoter`接口，可以简单的集成现有开源系统，比如：`zipkin`和`jaeger`。
