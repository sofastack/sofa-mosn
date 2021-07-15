## 使用 Seata 协调分布式事务

## 简介

+ 该样例工程演示了如何配置 Seata filter 与 Seata TC 交互对分布式事务进行协调。
+ 该 Seata filter 仅支持 http1 协议。
+ 被协调的 http rest 服务，try、confirm、cancel 对应的接口请求参数应一致。
+ 被协调的 http rest 服务，try、confirm、cancel 对应的接口必须是 POST 接口。

## 准备

需要一个编译好的 MOSN 程序
```
cd ${projectpath}/cmd/mosn/main
go build
```

+ 需要运行 seata tc server，地址：https://github.com/opentrx/seata-golang/tree/v2


+ 示例代码目录

```
${targetpath} = ${projectpath}/examples/codes/seata/
```

+ 将编译好的程序移动到示例代码目录

```
mv main ${targetpath}/
cd ${targetpath}
```

## 目录结构

```
main        // 编译完成的 MOSN 程序
|-- server_a
|-- |-- server_a.go 
|-- |-- server_a_config.json 
|-- server_b
|-- |-- server_b.go 
|-- |-- server_b_config.json 
|-- server_c
|-- |-- server_c.go 
|-- |-- server_c_config.json 
```

## 运行说明

### 启动 seata tc server

参考 https://github.com/opentrx/seata-golang/tree/v2

### 启动 MOSN

+ 使用 server_a_config.json 启动 MOSN

```
./main start -c server_a/server_a_config.json
```

+ 使用 server_b_config.json 启动 MOSN

```
./main start -c server_b/server_b_config.json
```

+ 使用 server_c_config.json 启动 MOSN

```
./main start -c server_c/server_c_config.json
```

### 启动 rest 服务

+ 启动 server_a.go
+ 启动 server_b.go
+ 启动 server_c.go

### 验证 Seata 的处理

+ 浏览器访问 http://localhost:2046/service-a/begin, tc 会自动调用 server_b、server_c 的 confirm 接口
