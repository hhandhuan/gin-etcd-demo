# gin 整合 etcd 的简单案例


## 使用说明

### ginserver 用户服务

服务启动首先将服务通信地址注册到 `etcd` 中

### ginclient 业务服务

业务服务从 `etcd` 中发现用户服务，使之与其通信


ps: 需要启动 etcd 服务才能测试该 demo，本案例以单节点 etcd 为运行环境