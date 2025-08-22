# caddy-nacos-client

`caddy-nacos-client` 是一个caddy插件，集成了nacos客户端，用来做服务发现

# 使用方法

## 编译

```
xcaddy.exe build master  --with github.com/li-zheng-hao/caddy-nacos=./
```

## 配置文件

```
{
	nacos {
		ip_addr ip_sample
		port port_sample
		namespace namespace_sample
		# 多个namespace时，使用namespaces
		# namespaces namespace1 namespace2 namespace3
		username username_sample
		password password_sample
	}
}

:8891 {
	reverse_proxy {
		dynamic nacosupstream {
			service_name service_name_sample
			group_name DEFAULT_GROUP
			namespace namespace_sample
			duration 30s
		}
		lb_policy least_conn
	}
}
```