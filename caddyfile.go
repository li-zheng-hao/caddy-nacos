package caddynacos

import (
    "github.com/caddyserver/caddy/v2"
    "github.com/caddyserver/caddy/v2/caddyconfig"
    "github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
    "github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
)

func init() {
    httpcaddyfile.RegisterGlobalOption("nacos", parseApp)
}

func parseApp(d *caddyfile.Dispenser, _ interface{}) (interface{}, error) {
    app := new(NacosClientApp)

    err := app.UnmarshalCaddyfile(d)

    return httpcaddyfile.App{
        Name:  "nacos",
        Value: caddyconfig.JSON(app, nil),
    }, err
}

// UnmarshalCaddyfile deserializes Caddyfile tokens into h.
//
//	nacos  {
//	    ip_addr             <ip>
//	    port           <port>
//	}
func (a *NacosClientApp) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
    for d.Next() {
        for nesting := d.Nesting(); d.NextBlock(nesting); {
            switch d.Val() {
            case "ip_addr":
                if !d.AllArgs(&a.IPAddr) {
                    return d.ArgErr()
                }
            case "port":
                if !d.AllArgs(&a.Port) {
                    return d.ArgErr()
                }
            default:
                return d.Errf("%s not a valid NacosClientApp option", d.Val())
            }
        }
    }

    return nil
}

// UnmarshalCaddyfile deserializes Caddyfile tokens into h.
// call by reverse_proxy dynamic
//	nacosupstream  {
//	    deration        <duration>
//	    service_name    <service_name>
//	}
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
            default:
                return d.Errf("unrecognized NacosUpstreams: %s", d.Val())
            }
        }
    }

    return nil
}
