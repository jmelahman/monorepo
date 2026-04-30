package docker

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// PortProxy listens on a host port and forwards to a container ip:port.
type PortProxy struct {
	HostPort      int
	ContainerIP   string
	ContainerPort int

	listener net.Listener
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewPortProxy(hostPort int, containerIP string, containerPort int) *PortProxy {
	return &PortProxy{HostPort: hostPort, ContainerIP: containerIP, ContainerPort: containerPort}
}

func (p *PortProxy) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p.HostPort))
	if err != nil {
		return fmt.Errorf("listen :%d: %w", p.HostPort, err)
	}
	p.listener = ln
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		<-ctx.Done()
		_ = ln.Close()
	}()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("proxy %d accept: %v", p.HostPort, err)
				return
			}
			go p.handle(ctx, conn)
		}
	}()
	return nil
}

func (p *PortProxy) handle(ctx context.Context, client net.Conn) {
	defer client.Close()
	target := fmt.Sprintf("%s:%d", p.ContainerIP, p.ContainerPort)
	dialer := net.Dialer{Timeout: 5 * time.Second}
	upstream, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		log.Printf("proxy %d dial %s: %v", p.HostPort, target, err)
		return
	}
	defer upstream.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()
	<-done
}

func (p *PortProxy) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}

// ProxyManager owns active proxies keyed by host port.
type ProxyManager struct {
	mu      sync.Mutex
	proxies map[int]*PortProxy
	ctx     context.Context
}

func NewProxyManager(ctx context.Context) *ProxyManager {
	return &ProxyManager{proxies: map[int]*PortProxy{}, ctx: ctx}
}

func (m *ProxyManager) Open(hostPort int, containerIP string, containerPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.proxies[hostPort]; ok {
		return fmt.Errorf("proxy on host port %d already running", hostPort)
	}
	p := NewPortProxy(hostPort, containerIP, containerPort)
	if err := p.Start(m.ctx); err != nil {
		return err
	}
	m.proxies[hostPort] = p
	return nil
}

func (m *ProxyManager) Close(hostPort int) {
	m.mu.Lock()
	p, ok := m.proxies[hostPort]
	if ok {
		delete(m.proxies, hostPort)
	}
	m.mu.Unlock()
	if p != nil {
		p.Stop()
	}
}

func (m *ProxyManager) CloseAll() {
	m.mu.Lock()
	all := make([]*PortProxy, 0, len(m.proxies))
	for _, p := range m.proxies {
		all = append(all, p)
	}
	m.proxies = map[int]*PortProxy{}
	m.mu.Unlock()
	for _, p := range all {
		p.Stop()
	}
}
