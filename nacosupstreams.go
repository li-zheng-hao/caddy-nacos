package caddynacos

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"go.uber.org/zap"
)

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
	Namespace   string   `json:"namespace,omitempty"`

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

func (nu *NacosUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {
	nu.logger.Info("GetUpstreams", zap.String("service_name", nu.ServiceName), zap.String("group_name", nu.GroupName), zap.String("namespace", nu.Namespace))
	instances, err := nu.nacosClient.SelectInstances(nu.ServiceName, nu.GroupName, []string{"DEFAULT"}, nu.Namespace)
	if err != nil {
		nu.logger.Error("GetUpstreams", zap.Error(err))
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

func (nu *NacosUpstreams) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "duration":
				if !d.NextArg() {
					return d.ArgErr()
				}
				dur, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return d.Errf("parsing refresh interval duration: %v", err)
				}
				nu.Refresh = caddy.Duration(dur)
			case "service_name":
				if !d.AllArgs(&nu.ServiceName) {
					return d.ArgErr()
				}
			case "group_name":
				if !d.AllArgs(&nu.GroupName) {
					return d.ArgErr()
				}
			case "namespace":
				if !d.AllArgs(&nu.Namespace) {
					return d.ArgErr()
				}
			default:
				return d.Errf("unrecognized NacosUpstreams directive: %s", d.Val())
			}
		}
	}

	return nil
}

// Interface guards
var (
	_ caddy.Provisioner           = (*NacosUpstreams)(nil)
	_ reverseproxy.UpstreamSource = (*NacosUpstreams)(nil)
	_ caddyfile.Unmarshaler       = (*NacosUpstreams)(nil)
)
