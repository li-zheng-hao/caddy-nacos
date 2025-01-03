package caddynacos

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"go.uber.org/zap"
)

const defaultRefreshDureation = 30

func init() {
	caddy.RegisterModule(NacosUpstreams{})
}

type NacosUpstreams struct {
	// The interval at which to refresh the nacos lookup.
	// Results are cached between lookups. Default: 1m
	Refresh caddy.Duration `json:"refresh,omitempty"`

	// service name for search ip:port service from nacos server
	ServiceName string   `json:"service_name,omitempty"`
	GroupName   string   `json:"group_name,omitempty"`
	Cluster     []string `json:"cluster,omitempty"`

	//nacos cient to search instances for dynamic upstream
	nacosClient *NacosClientApp
	logger      *zap.Logger
}

func (NacosUpstreams) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.upstreams.nacosupstream",
		New: func() caddy.Module { return new(NacosUpstreams) },
	}
}

func (nu *NacosUpstreams) Provision(ctx caddy.Context) error {
	nu.logger = ctx.Logger(nu)
	if nu.Refresh == 0 {
		nu.Refresh = caddy.Duration(time.Minute)
	}

	nacosAppIface, err := ctx.App("nacos")
	if err != nil {
		return fmt.Errorf("getting nacos app: %v", err)
	}
	nu.nacosClient = nacosAppIface.(*NacosClientApp)

	return nil
}

func (nu NacosUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {

	instances, err := nu.nacosClient.SelectInstances(nu.ServiceName, nu.GroupName, []string{"DEFAULT"})
	if err != nil {
		// From SelectInstances docs: "If the response contains invalid names, those records are filtered
		// out and an error will be returned alongside the remaining results, if any." Thus, we
		// only return an error if no records were also returned.
		if len(instances) == 0 {
			return nil, err
		}
		nu.logger.Warn("NACOS instances filtered", zap.Error(err))
	}

	nu.logger.Info("discovered NACOS instance", zap.Int("len", len(instances)))
	upstreams := make([]*reverseproxy.Upstream, len(instances))
	for i, ins := range instances {
		nu.logger.Debug("discovered NACOS instance",
			zap.String("target", ins.Ip),
			zap.Uint64("port", ins.Port))
		addr := net.JoinHostPort(ins.Ip, strconv.Itoa(int(ins.Port)))
		upstreams[i] = &reverseproxy.Upstream{Dial: addr}
	}

	return upstreams, nil
}

type nacosLookups struct {
	nacosUpstreams NacosUpstreams
	freshness      time.Time
	upstreams      []*reverseproxy.Upstream
}

func (nl nacosLookups) isFresh() bool {
	return time.Since(nl.freshness) < time.Duration(nl.nacosUpstreams.Refresh)
}

var (
	nacosSrvs = make(map[string]nacosLookups)
	nacosMu   sync.RWMutex
)

//Interface guards
var (
	_ caddy.Provisioner           = (*NacosUpstreams)(nil)
	_ reverseproxy.UpstreamSource = (*NacosUpstreams)(nil)
)
