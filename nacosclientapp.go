package caddynacos

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"

	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(NacosClientApp{})
}

// NacosClientApp provides upstreams from nacos server.
type NacosClientApp struct {
	//IPAddr specifies the ip for nacos server
	IPAddr string `json:"ip_addr,omitempty"`

	//Port specifies the port for nacos server
	Port string `json:"port,omitempty"`

	//Namespaces specifies the namespaces for nacos server
	Namespaces []string `json:"namespaces,omitempty"`

	//Username specifies the username for nacos server authentication
	Username string `json:"username,omitempty"`

	//Password specifies the password for nacos server authentication
	Password string `json:"password,omitempty"`

	//nacos clinet for naming service search
	nacosClients map[string]naming_client.INamingClient
	clientLock   *sync.RWMutex
	nacosSC      []constant.ServerConfig

	logger *zap.Logger
	ctx    caddy.Context
}

// ServiceInstance is cache for searched instance
type ServiceInstance struct {
	serviceName string
	instance    []model.Instance
}

// CaddyModule returns the Caddy module information.
func (NacosClientApp) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "nacos",
		New: func() caddy.Module { return new(NacosClientApp) },
	}
}

func (app *NacosClientApp) Provision(ctx caddy.Context) error {
	app.ctx = ctx
	app.logger = ctx.Logger(app)
	app.nacosClients = make(map[string]naming_client.INamingClient)
	app.clientLock = &sync.RWMutex{}

	// 通过上下文获取nacos应用配置
	ipAddr, portStr, namespaces, username, password := GetGlobalConfig()

	// 从nacos应用实例中复制配置
	app.IPAddr = ipAddr
	app.Port = portStr
	app.Namespaces = make([]string, len(namespaces))
	copy(app.Namespaces, namespaces)
	app.Username = username
	app.Password = password

	// 添加详细的配置日志
	app.logger.Info("nacos app provision started",
		zap.String("ipaddr", app.IPAddr),
		zap.String("port", app.Port),
		zap.Strings("namespaces", app.Namespaces),
		zap.String("username", app.Username),
		zap.String("password", app.Password))

	// 添加调试信息，包括对象地址
	fmt.Printf("DEBUG: Provision called - Object: %p, IPAddr: %s, Port: %s, Namespaces: %v, Username: %s, Password: %s\n",
		app, app.IPAddr, app.Port, app.Namespaces, app.Username, app.Password)

	// create Server Config
	app.logger.Info("nacos server addr", zap.String("ipaddr", app.IPAddr), zap.String("port", app.Port))
	port, _ := strconv.ParseUint(app.Port, 0, 64)
	app.nacosSC = []constant.ServerConfig{
		*constant.NewServerConfig(app.IPAddr, port),
	}

	app.logger.Info("nacos app provision completed successfully")
	return nil
}

func (app *NacosClientApp) SelectInstances(sname string, gname string, cluster []string, namespace string) ([]model.Instance, error) {
	app.clientLock.RLock()
	client, ok := app.nacosClients[namespace]
	app.clientLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("nacos client for namespace '%s' not found", namespace)
	}

	skey := namespace + "@@" + gname + "@@" + sname
	param := vo.SelectInstancesParam{
		ServiceName: sname,
		GroupName:   gname,
		HealthyOnly: true,
		// Clusters:    cluster,
	}
	instance, err := client.SelectInstances(param)
	if err != nil {
		app.logger.Error("SelectInstances", zap.Error(err))
		return nil, fmt.Errorf("SelectInstances %v failed: %w", skey, err)
	}
	instances := make([]model.Instance, 0)
	for _, instance := range instance {
		// 过滤掉权重小于1的实例
		if instance.Weight > 0 {
			instances = append(instances, instance)
		}
	}

	subscribeMu.Lock()
	_, ok = subscribeFlag[skey]
	if !ok {
		subscribeFlag[skey] = true
		go func(app *NacosClientApp, client naming_client.INamingClient) {
			app.logger.Info("nacos subscribe", zap.String("ServiceName", sname), zap.String("GroupName", gname), zap.String("namespace", namespace))
			param1 := &vo.SubscribeParam{
				ServiceName: sname,
				GroupName:   gname,
				SubscribeCallback: func(services []model.Instance, err error) {
					app.logger.Info("nacos client Subscribe call back", zap.String("ServiceName", sname), zap.String("GroupName", gname), zap.String("namespace", namespace), zap.Int("instance len", len(services)))
				},
			}
			err = client.Subscribe(param1)
			if err != nil {
				app.logger.Error("nacos client Subscribe call back failed!")
			}
		}(app, client)
	}
	subscribeMu.Unlock()

	app.logger.Info("nacos SelectInstances", zap.String("ServiceName", sname), zap.String("GroupName", gname), zap.String("namespace", namespace), zap.Int("instance len", len(instances)))
	return instances, nil
}

func (app *NacosClientApp) SelectInstancesWithCache(sname string, gname string, cluster []string, namespace string) ([]model.Instance, error) {
	app.clientLock.RLock()
	client, ok := app.nacosClients[namespace]
	app.clientLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("nacos client for namespace '%s' not found", namespace)
	}

	skey := namespace + "@@" + gname + "@@" + sname
	sinstance := ServiceInstance{
		serviceName: skey,
	}

	//先尝试直接从缓存中获取服务
	serviceMu.RLock()
	value, ok := serviceInstance[skey]
	serviceMu.RUnlock()
	if ok {
		app.logger.Info("nacos client SelectInstances from cache", zap.String("cache key", skey))
		return value.instance, nil
	}

	//缓存中不存在时，首次调用获取接口获取
	param := vo.SelectInstancesParam{
		ServiceName: sname,
		GroupName:   gname,
		Clusters:    cluster,
		HealthyOnly: true,
	}
	instances, err := client.SelectInstances(param)
	if err != nil {
		return nil, fmt.Errorf("SelectInstances %v failed: %w", skey, err)
	}
	sinstance.instance = instances

	app.logger.Info("nacos client set instance cache", zap.Int("instance len", len(instances)))
	serviceMu.Lock()
	serviceInstance[skey] = sinstance
	serviceMu.Unlock()

	//缓存中没有时，表示没有获取过，需要subscribe监听一下
	param1 := &vo.SubscribeParam{
		ServiceName: sname,
		GroupName:   gname,
		SubscribeCallback: func(services []model.Instance, err error) {
			app.logger.Info("nacos client Subscribe call back", zap.Int("instance len", len(services)))
			if len(services) > 0 {
				name := services[0].ServiceName
				ss := ServiceInstance{
					serviceName: name,
					instance:    services,
				}
				serviceMu.Lock()
				serviceInstance[name] = ss
				app.logger.Info("nacos client update instance cache", zap.Int("instance len", len(serviceInstance[name].instance)))
				serviceMu.Unlock()
			}
		},
	}
	err = client.Subscribe(param1)
	if err != nil {
		return nil, fmt.Errorf("subscribe %v failed: %w", skey, err)
	} else {
		app.logger.Info("nacos subscribe success")
	}

	return instances, nil
}

func (app *NacosClientApp) Start() error {
	// connect to the nacos server
	app.logger.Info("connecting to nacos server for namespaces", zap.Strings("namespaces", app.Namespaces))

	if len(app.Namespaces) == 0 {
		// Add default empty namespace if none are configured
		app.Namespaces = append(app.Namespaces, "")
	}

	for _, ns := range app.Namespaces {
		cc := constant.NewClientConfig(
			constant.WithNamespaceId(ns),
			constant.WithTimeoutMs(5000),
			constant.WithNotLoadCacheAtStart(true),
			constant.WithLogLevel("debug"),
			constant.WithUsername(app.Username),
			constant.WithPassword(app.Password),
		)
		client, err := clients.NewNamingClient(
			vo.NacosClientParam{
				ClientConfig:  cc,
				ServerConfigs: app.nacosSC,
			},
		)
		if err != nil {
			app.logger.Error("connection to nacos server failed for namespace", zap.String("namespace", ns), zap.Error(err))
			return err
		}
		app.clientLock.Lock()
		app.nacosClients[ns] = client
		app.clientLock.Unlock()
		app.logger.Info("created nacos client for namespace", zap.String("namespace", ns))
	}

	return nil
}

func (app *NacosClientApp) Stop() error {
	app.logger.Info("closing nacos connection")
	return nil
}

// Interface guards
var (
	_ caddy.App             = (*NacosClientApp)(nil)
	_ caddy.Provisioner     = (*NacosClientApp)(nil)
	_ caddyfile.Unmarshaler = (*NacosClientApp)(nil)
)

var (
	serviceInstance = make(map[string]ServiceInstance)
	serviceMu       sync.RWMutex

	//是否订阅的flag
	subscribeFlag = make(map[string]bool)
	subscribeMu   sync.RWMutex
)
