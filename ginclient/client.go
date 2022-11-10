package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	clientV3 "go.etcd.io/etcd/client/v3"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// 该服务仅测试用户服务

// serverPrefix 服务器注册前缀 key
var serverPrefix = "services"

// serverName 用户服务 key
var serverName = "user"

// etcdEndpoint etcd 服务地址
var etcdEndpoint = "127.0.0.1:2379"

// locker 全局服务锁
var locker = sync.Mutex{}

// services 服务入口列表
var services = map[string]map[string]string{
	serverName: {},
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.Default()

	go func() {
		serviceDiscovery()
	}()

	engine.GET("/test", func(c *gin.Context) {
		key, endpoint, err := getService("user")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"msg": err.Error(), "code": -1, "data": nil})
			return
		}
		log.Printf("get servie success, key: %v, endpoint: %v", key, endpoint)

		resp, err := http.Get(fmt.Sprintf("http://%s/user/info", endpoint))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"msg": err.Error(), "code": -1, "data": nil})
			return
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"msg": err.Error(), "code": -1, "data": nil})
			return
		}

		var data gin.H
		json.Unmarshal(body, &data)

		c.JSON(http.StatusOK, gin.H{"data": data, "msg": "ok", "code": 0})
	})
	if err := engine.Run("127.0.0.1:8787"); err != nil {
		log.Printf("gin run error: %v", err)
	}
}

// serviceDiscovery 服务发现
func serviceDiscovery() {
	client, err := clientV3.New(clientV3.Config{
		Endpoints:            []string{etcdEndpoint}, // etcd 服务仅单机节点
		DialTimeout:          time.Second * 10,       // 与 etcd 服务建立的超时时间
		DialKeepAliveTimeout: time.Second * 30,       // 客户端等待保持连接探测响应的时间。如果在此期间没有收到响应，则连接将关闭
	})

	if err != nil {
		log.Printf("etcd server conn error: %v", err)
		return
	}

	for serviceName, _ := range services {
		go func(serviceName string) {
			ctx := context.Background()
			serviceKey := fmt.Sprintf("%s/%s", serverPrefix, serviceName)
			// 获取当前所有服务入口
			getRes, err := client.Get(ctx, serviceKey, clientV3.WithPrefix())
			if err != nil {
				log.Printf("get response error: %v", err)
				return
			}
			// 防止并发写
			locker.Lock()
			for _, v := range getRes.Kvs {
				services[serviceName][string(v.Key)] = string(v.Value)
			}
			locker.Unlock()
			// 监听 key 变化
			ch := client.Watch(ctx, serviceKey, clientV3.WithPrefix(), clientV3.WithPrevKV())
			for v := range ch {
				for _, v := range v.Events {
					key := string(v.Kv.Key)
					value := string(v.Kv.Value)

					// 上一个修订版本的值
					preValue := ""
					if v.PrevKv != nil {
						preValue = string(v.PrevKv.Value)
					}

					switch v.Type {
					case 0: // PUT，新增或替换
						locker.Lock()
						services[serviceName][key] = value
						locker.Unlock()
						fmt.Printf("service %s put endpoint, key: %s, endpoint: %s\n", serviceName, key, value)
					case 1: // 删除
						locker.Lock()
						delete(services[serviceName], key)
						locker.Unlock()
						fmt.Printf("service %s delete endpoint, key: %s, endpoint: %s\n", serviceName, key, preValue)
					}
				}
			}
		}(serviceName)
	}
}

// getService 获取服务
// name 服务名字
func getService(name string) (key string, endpoint string, err error) {
	endpoints := services[name]
	if len(endpoints) == 0 {
		return "", "", errors.New(fmt.Sprintf("%s服务不可用，请稍后再试", name))
	}

	keys := make([]string, 0)
	for v := range endpoints {
		keys = append(keys, v)
	}

	randomKey := keys[rand.Intn(len(keys))] // 模拟负载均衡

	return randomKey, endpoints[randomKey], nil
}
