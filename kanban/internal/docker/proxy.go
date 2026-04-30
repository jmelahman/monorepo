package docker

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// PortProxy listens on a host port and forwards each connection through a
// `docker exec <container> socat - TCP:127.0.0.1:<port>` pipe. Routing through
// the container's loopback sidesteps the firewall rules in the target image,
// which already allow inbound on `lo`.
type PortProxy struct {
	HostPort      int
	ContainerID   string
	ContainerPort int
	client        *Client

	listener net.Listener
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewPortProxy(client *Client, hostPort int, containerID string, containerPort int) *PortProxy {
	return &PortProxy{
		HostPort:      hostPort,
		ContainerID:   containerID,
		ContainerPort: containerPort,
		client:        client,
	}
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

	cmd := []string{"socat", "-", fmt.Sprintf("TCP:127.0.0.1:%d", p.ContainerPort)}
	resp, err := p.client.cli.ContainerExecCreate(ctx, p.ContainerID, container.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		log.Printf("proxy %d exec create: %v", p.HostPort, err)
		return
	}
	att, err := p.client.cli.ContainerExecAttach(ctx, resp.ID, container.ExecStartOptions{})
	if err != nil {
		log.Printf("proxy %d exec attach: %v", p.HostPort, err)
		return
	}
	defer att.Close()

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(att.Conn, client)
		_ = att.CloseWrite()
		done <- struct{}{}
	}()
	go func() {
		_, _ = stdcopy.StdCopy(client, io.Discard, att.Reader)
		done <- struct{}{}
	}()
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
	client  *Client
}

func NewProxyManager(ctx context.Context, client *Client) *ProxyManager {
	return &ProxyManager{proxies: map[int]*PortProxy{}, ctx: ctx, client: client}
}

func (m *ProxyManager) Open(hostPort int, containerID string, containerPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.proxies[hostPort]; ok {
		return fmt.Errorf("proxy on host port %d already running", hostPort)
	}
	if containerID == "" {
		return fmt.Errorf("container not running")
	}
	p := NewPortProxy(m.client, hostPort, containerID, containerPort)
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
