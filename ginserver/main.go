package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	clientV3 "go.etcd.io/etcd/client/v3"
)

// port gin http port
var port int

// prefix 服务器注册key前缀
var prefix = "services"

// etcdEndpoint etcd 服务地址
var etcdEndpoint = "127.0.0.1:2379"

// serverName 该服务名字 该服务仅为用户服务
var serverName = "user"

// serverHost gin 服务地址 注意这里仅仅本地测试地址
var serverHost = "127.0.0.1"

func main() {
	flag.IntVar(&port, "servicePort", 8088, "")
	flag.Parse()

	go func() {
		UserServiceRegistry()
	}()

	engine := gin.Default()
	// 获取用户信息接口
	engine.GET("/user/info", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user":   gin.H{"username": "gin-etcd-demo"},
			"server": gin.H{"host": serverHost, "port": port},
		})
	})

	if err := engine.Run(fmt.Sprintf("%s:%d", serverHost, port)); err != nil {
		log.Printf("gin run error: %v", err)
	}
}

// UserServiceRegistry 用户服务注册
func UserServiceRegistry() {
	client, err := clientV3.New(clientV3.Config{
		Endpoints:            []string{etcdEndpoint}, // etcd 服务仅单机节点
		DialTimeout:          time.Second * 30,       // 与 etcd 服务建立的超时时间
		DialKeepAliveTimeout: time.Second * 30,       // 客户端等待保持连接探测响应的时间。如果在此期间没有收到响应，则连接将关闭
	})

	if err != nil {
		log.Printf("etcd server conn error: %v", err)
		return
	}

	key := fmt.Sprintf("%s/%s/%d", prefix, serverName, port)
	val := fmt.Sprintf("%s:%d", serverHost, port)

	ctx := context.Background()

	ttl := 10 // 租约10秒钟

	leaseResponse, err := client.Grant(ctx, int64(ttl))
	if err != nil {
		log.Printf("etcd ceate lease error: %v", err)
		return
	}

	log.Printf("crate lease %v", leaseResponse)

	putResponse, err := client.Put(ctx, key, val, clientV3.WithLease(leaseResponse.ID))
	if err != nil {
		log.Printf("etcd lease put error: %v", err)
		return
	}

	log.Printf("put response %v", putResponse)

	// 保持租约不过期
	klRes, err := client.KeepAlive(ctx, leaseResponse.ID)
	if err != nil {
		panic(err)
	}

	// 监听续约情况
	for v := range klRes {
		b, _ := json.Marshal(v)
		fmt.Printf("[%v] keep lease alive suucess: %s\n", time.Now(), string(b))
	}

	fmt.Println("stop keeping lease alive")
}
