package k8sresolver

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
)

const (
	defaultPort      = "443"
	defaultNamespace = "default"
	minK8SResRate    = 5 * time.Second
)

var logger = grpclog.Component("k8s")

func init() {
	resolver.Register(NewBuilder())
}

// NewBuilder creates a k8sBuilder which is used to factory K8S service resolvers.
func NewBuilder() resolver.Builder {
	return &k8sBuilder{}
}

type k8sBuilder struct{}

func (b *k8sBuilder) Build(
	target resolver.Target,
	cc resolver.ClientConn,
	opts resolver.BuildOptions,
) (resolver.Resolver, error) {
	host, port, err := parseTarget(target.Endpoint(), defaultPort)
	if err != nil {
		return nil, err
	}

	namespace, host := getNamespaceFromHost(host)

	k8sc, err := newInClusterClient(namespace)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	k := &k8sResolver{
		k8sC:   k8sc,
		host:   host,
		port:   port,
		ctx:    ctx,
		cancel: cancel,
		cc:     cc,
		rn:     make(chan struct{}, 1),
	}

	k.wg.Add(1)
	go k.watcher()
	k.ResolveNow(resolver.ResolveNowOptions{})
	return k, nil
}

// Scheme returns the naming scheme of this resolver builder, which is "k8s".
func (b *k8sBuilder) Scheme() string {
	return "k8s"
}

func getNamespaceFromHost(host string) (string, string) {
	namespace := defaultNamespace

	hostParts := strings.Split(host, ".")
	if len(hostParts) >= 2 {
		namespace = hostParts[1]
		host = hostParts[0]
	}
	return namespace, host
}
