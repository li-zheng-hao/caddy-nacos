package caddynacos

import (
    "fmt"
    "github.com/caddyserver/caddy/v2"
    "github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
    "strconv"
    "sync"

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
    IPAddr string `json:"ip_addr, omitempty"`

    //Port specifies the port for nacos server
    Port string `json:"port, omitempty"`

    //nacos clinet for naming service search
    nacosClient naming_client.INamingClient
    nacosSC     []constant.ServerConfig
    nacosCC     *constant.ClientConfig

    logger *zap.Logger
    ctx    caddy.Context
}

//ServiceInstance is cache for searched instance
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

func (app *NacosClientApp) CreateNacosCient() error {
    // create naming client
    var err error
    app.nacosClient, err = clients.NewNamingClient(
        vo.NacosClientParam{
            ClientConfig:  app.nacosCC,
            ServerConfigs: app.nacosSC,
        },
    )
    if err != nil {
        return err
    }

    return nil
}

func (app *NacosClientApp) Provision(ctx caddy.Context) error {
    app.ctx = ctx
    app.logger = ctx.Logger(app)

    // create Server Config
    app.logger.Info("nacos server addr", zap.String("ipaddr", app.IPAddr), zap.String("port", app.Port))
    port, _ := strconv.ParseUint(app.Port, 0, 64)
    app.nacosSC = []constant.ServerConfig{
        *constant.NewServerConfig(app.IPAddr, port, constant.WithContextPath("/nacos")),
    }

    // create Client Config
    app.nacosCC = constant.NewClientConfig(
        constant.WithNamespaceId(""),
        constant.WithTimeoutMs(5000),
        constant.WithNotLoadCacheAtStart(true),
        constant.WithLogLevel("debug"),
    )

    return nil
}

func (app *NacosClientApp) SelectInstances(sname string, gname string, cluster []string) ([]model.Instance, error) {
    skey := gname + "@@" + sname
    param := vo.SelectInstancesParam{
        ServiceName: sname,
        GroupName:   gname,
        //Clusters:    cluster,
        HealthyOnly: true,
    }
    instances, err := app.nacosClient.SelectInstances(param)
    if err != nil {
        return nil, fmt.Errorf("SelectInstances %v failed!", skey)
    }

    subscribeMu.Lock()
    _, ok := subscribeFlag[skey]
    if ok == false {
        subscribeFlag[skey] = true
        go func(app *NacosClientApp) {
            app.logger.Info("nacos subscribe", zap.String("ServiceName", sname), zap.String("GroupName", gname))
            param1 := &vo.SubscribeParam{
                ServiceName: sname,
                GroupName:   gname,
                SubscribeCallback: func(services []model.Instance, err error) {
                    app.logger.Info("nacos client Subscribe call back", zap.String("ServiceName", sname), zap.String("GroupName", gname), zap.Int("instance len", len(services)))
                },
            }
            err = app.nacosClient.Subscribe(param1)
            if err != nil {
                app.logger.Error("nacos client Subscribe call back failed!")
            }
        }(app)
    }
    subscribeMu.Unlock()

    app.logger.Info("nacos SelectInstances", zap.String("ServiceName", sname), zap.String("GroupName", gname), zap.Int("instance len", len(instances)))
    return instances, nil
}

func (app *NacosClientApp) SelectInstancesWithCache(sname string, gname string, cluster []string) ([]model.Instance, error) {
    skey := gname + "@@" + sname
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
    instances, err := app.nacosClient.SelectInstances(param)
    if err != nil {
        return nil, fmt.Errorf("SelectInstances %v failed!", skey)
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
    err = app.nacosClient.Subscribe(param1)
    if err != nil {
        return nil, fmt.Errorf("Subscribe %v failed!", skey)
    } else {
        app.logger.Info("nacos subscribe success")
    }

    //app.logger.Info("SelectInstances,param:%+v, result:%+v \n\n", param, instances)
    return instances, nil
}

func (app *NacosClientApp) Start() error {
    // connect to the nacos server
    app.logger.Info("connecting to nacos server")
    err := app.CreateNacosCient()
    if err != nil {
        app.logger.Error("connection to nacos server failed", zap.Error(err))
        panic(err)
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
