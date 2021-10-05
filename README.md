# Golang 配置中心

## 特性

* 纯Golang实现
* 基于Raft协议

## TODO

* 移除节点（下线）

## 初始化集群

### 启动第一个节点

```bash
go run ./cmd/main.go -id node-1 -haddr localhost:8081 -raddr localhost:9001 \
-rdir ~/data/go-config-center/node1
```

### 启动两个节点，并加入第一个节点的集群中

```bash
go run ../cmd/main.go -id node-2 -haddr localhost:8082 -raddr localhost:9002 \
-rdir ~/data/go-config-center/node2 -join localhost:8081

go run ../cmd/main.go -id node-3 -haddr localhost:8083 -raddr localhost:9003 \
-rdir ~/data/go-config-center/node3 -join localhost:8081
```

## 启动集群

> 初始化后，启动集群就无需join参数

```bash
go run ./cmd/main.go -id node-1 -haddr localhost:8081 -raddr localhost:9001 \
-rdir ~/data/go-config-center/node1

go run ../cmd/main.go -id node-2 -haddr localhost:8082 -raddr localhost:9002 \
-rdir ~/data/go-config-center/node2

go run ../cmd/main.go -id node-3 -haddr localhost:8083 -raddr localhost:9003 \
-rdir ~/data/go-config-center/node3
```

## 操作数据

> 分别在不同的节点上操作数据，根据需要会根据一致性级别自动重定向到Leader节点

### 添加数据

```bash
curl -X POST http://localhost:8081/key -d '{"foo": "bar"}' -L
curl -X GET http://localhost:8081/key/foo -L
```

### 删除数据

```bash
curl -X DELETE http://localhost:8081/key/foo -L
```

### 读取数据

```bash
curl -X GET http://localhost:8083/key/foo?level=stale
curl -X GET http://localhost:8082/key/foo?level=default  -L
curl -X GET http://localhost:8081/key/foo?level=consistent  -L
```
