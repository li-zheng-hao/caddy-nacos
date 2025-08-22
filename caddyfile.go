package caddynacos

import (
	"fmt"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
)

// 全局配置存储（在caddyfile.go中定义）
var (
	globalIPAddr     string
	globalPort       string
	globalNamespaces []string
	globalUsername   string
	globalPassword   string
)

// GetGlobalConfig 返回全局配置
func GetGlobalConfig() (string, string, []string, string, string) {
	return globalIPAddr, globalPort, globalNamespaces, globalUsername, globalPassword
}

func init() {
	// 注册nacos指令，告诉Caddy如何解析Caddyfile中的nacos块
	httpcaddyfile.RegisterGlobalOption("nacos", parseApp)
}

// 解析nacos配置块
func parseApp(d *caddyfile.Dispenser, _ interface{}) (interface{}, error) {
	app := new(NacosClientApp)

	err := app.UnmarshalCaddyfile(d)
	if err != nil {
		return nil, err
	}

	// 添加调试日志，显示解析后的对象状态
	fmt.Printf("DEBUG: parseApp completed - IPAddr: %s, Port: %s, Namespaces: %v, Username: %s, Password: %s\n",
		app.IPAddr, app.Port, app.Namespaces, app.Username, app.Password)

	// 将配置存储到全局变量中
	globalIPAddr = app.IPAddr
	globalPort = app.Port
	globalNamespaces = make([]string, len(app.Namespaces))
	copy(globalNamespaces, app.Namespaces)
	globalUsername = app.Username
	globalPassword = app.Password

	fmt.Printf("DEBUG: config stored to global variables\n")

	return app, nil
}

// UnmarshalCaddyfile deserializes Caddyfile tokens into h.
//
//	nacos {
//	    ip_addr      <ip>
//	    port         <port>
//	    namespace    <namespace1> [<namespace2>...]
//	    username     <username>
//	    password     <password>
//	}
func (a *NacosClientApp) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	fmt.Printf("DEBUG: UnmarshalCaddyfile\n")
	for d.Next() {
		// 解析nacos块内的配置
		for d.NextBlock(0) {
			directive := d.Val()
			switch directive {
			case "ip_addr":
				if !d.NextArg() {
					return d.ArgErr()
				}
				a.IPAddr = d.Val()
				// 添加调试日志
				fmt.Printf("DEBUG: parsed ip_addr = %s\n", a.IPAddr)
			case "port":
				if !d.NextArg() {
					return d.ArgErr()
				}
				a.Port = d.Val()
				// 添加调试日志
				fmt.Printf("DEBUG: parsed port = %s\n", a.Port)
			case "namespace":
				// Support multiple namespaces
				for d.NextArg() {
					ns := d.Val()
					a.Namespaces = append(a.Namespaces, ns)
					// 添加调试日志
					fmt.Printf("DEBUG: parsed namespace = %s\n", ns)
				}
			case "namespaces":
				// Support multiple namespaces
				for d.NextArg() {
					ns := d.Val()
					a.Namespaces = append(a.Namespaces, ns)
					// 添加调试日志
					fmt.Printf("DEBUG: parsed namespace = %s\n", ns)
				}
			case "username":
				if !d.NextArg() {
					return d.ArgErr()
				}
				a.Username = d.Val()
				// 添加调试日志
				fmt.Printf("DEBUG: parsed username = %s\n", a.Username)
			case "password":
				if !d.NextArg() {
					return d.ArgErr()
				}
				a.Password = d.Val()
				// 添加调试日志
				fmt.Printf("DEBUG: parsed password = %s\n", a.Password)
			default:
				return d.Errf("%s not a valid NacosClientApp option", directive)
			}
		}
	}

	return nil
}
