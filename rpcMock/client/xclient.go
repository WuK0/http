package client

import (
	"context"
	"io"
	"reflect"
	"rpcMock/discovery"
	"rpcMock/server"
	"sync"
)

var _ io.Closer = (*XClient)(nil)

type XClient struct {
	d       discovery.Discovery
	clients map[string]*Client
	opt     *server.Option
	mu      sync.Mutex
	mode    discovery.SelectMode
}

func NewXClient(d discovery.Discovery, mode discovery.SelectMode, opt *server.Option) *XClient {
	return &XClient{d: d, mode: mode, opt: opt, clients: make(map[string]*Client)}
}

func (c *XClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, client := range c.clients {
		_ = client.Close()
		delete(c.clients, key)
	}
	return nil
}

func (c *XClient) dial(rpcAddr string) (*Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	client, ok := c.clients[rpcAddr]
	if ok && !client.IsAvailable() {
		_ = client.Close()
		delete(c.clients, rpcAddr)
		client = nil
	}
	if client == nil {
		// 新建连接
		var err error
		client, err = XDial(rpcAddr, c.opt)
		if err != nil {
			return nil, err
		}
		c.clients[rpcAddr] = client
	}
	return client, nil
}

func (c *XClient) call(rpcAddr string, ctx context.Context, serviceMethod string, args, reply interface{}) error {
	client, err := c.dial(rpcAddr)
	if err != nil {
		return err
	}
	return client.Call(ctx, serviceMethod, args, reply)
}

func (c *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	rpcAddr, err := c.d.Get(c.mode)
	if err != nil {
		return err
	}
	return c.call(rpcAddr, ctx, serviceMethod, args, reply)
}

// Broadcast 将请求广播到所有的服务实例，如果任意一个实例发生错误，则返回其中一个错误；如果调用成功，则返回其中一个的结果
func (c *XClient) Broadcast(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var err error
	servers, err := c.d.GetAll()
	if err != nil {
		return err
	}
	replyDone := reply == nil
	ctx, cancel := context.WithCancel(ctx)
	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var cloneReply interface{}
			if reply != nil {
				cloneReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			e := c.call(rpcAddr, ctx, serviceMethod, args, cloneReply)
			mu.Lock()
			if e != nil && err == nil {
				err = e
				cancel()
			}

			if err == nil && !replyDone {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(cloneReply).Elem())
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}
	wg.Wait()
	cancel()
	return err

}
