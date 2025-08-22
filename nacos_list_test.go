package caddynacos

import (
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

func TestHelloWorld(t *testing.T) {
	godotenv.Load()
	// 创建clientConfig的另一种方式
	clientConfig := *constant.NewClientConfig(
		constant.WithNamespaceId(os.Getenv("NACOS_NAMESPACE_ID")), //当namespace是public时，此处填空字符串。
		constant.WithTimeoutMs(5000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogLevel("debug"),
		constant.WithUsername(os.Getenv("NACOS_USERNAME")),
		constant.WithPassword(os.Getenv("NACOS_PASSWORD")),
	)

	// 至少一个ServerConfig
	serverConfigs := []constant.ServerConfig{
		{
			IpAddr:      os.Getenv("NACOS_IP_ADDR"),
			ContextPath: "/nacos",
			Port:        8848,
			Scheme:      "http",
		},
	}

	client, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	services, err := client.GetService(vo.GetServiceParam{
		ServiceName: os.Getenv("NACOS_SERVICE_NAME"),
		Clusters:    []string{"DEFAULT"}, // 默认值DEFAULT
		GroupName:   "DEFAULT_GROUP",     // 默认值DEFAULT_GROUP
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(services)
	fmt.Println(services.Name)
	fmt.Println(services.GroupName)
	fmt.Println(services.Clusters)
	fmt.Println(services.Hosts)

	instance, err := client.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{
		ServiceName: os.Getenv("NACOS_SERVICE_NAME"),
		GroupName:   "DEFAULT_GROUP",
	})
	if err != nil {
		t.Fatal(err)
	}
	if instance == nil {
		t.Fatal("no instances found")
	}
	fmt.Println(instance)
	fmt.Println(instance.Ip)
	fmt.Println(instance.Port)
	fmt.Println(instance.Weight)
	fmt.Println(instance.Healthy)
	fmt.Println(instance.Enable)
	fmt.Println(instance.Metadata)
	fmt.Println(instance.ClusterName)
	fmt.Println(instance.ServiceName)
	fmt.Println(instance.Ephemeral)
}
