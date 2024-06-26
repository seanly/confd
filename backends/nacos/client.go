package nacos

import (
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/kelseyhightower/confd/log"
	"fmt"
	"strings"
	"net/url"
	"strconv"
)

var replacer = strings.NewReplacer("/", ".")

type Client struct {
	client config_client.IConfigClient
	group string
	namespace string
	channel chan int
}

func NewNacosClient(nodes []string, group string, config constant.ClientConfig) (client *Client, err error) {
	var configClient config_client.IConfigClient
	servers := []constant.ServerConfig{
	}
	for _, key := range nodes {
		nacosUrl,_ := url.Parse(key)

		port, _ := strconv.Atoi(nacosUrl.Port())
		servers = append(servers, constant.ServerConfig{
			IpAddr: nacosUrl.Hostname(),
			Port:   uint64(port),
		})
	}

	if len(strings.TrimSpace(group)) == 0 {
		group = "DEFAULT_GROUP"
	}

	log.Info(fmt.Sprintf("namespace=%s, group=%s", config.NamespaceId, group))

	configClient, err = clients.CreateConfigClient(map[string]interface{}{
		"serverConfigs": servers,
		"clientConfig": constant.ClientConfig{
			TimeoutMs:           10000,
			ListenInterval:      20000,
			NotLoadCacheAtStart: true,
			NamespaceId:	     config.NamespaceId,
			Username: 			 config.Username,
			Password: 			 config.Password,
		},
	})

	client = &Client{configClient, group, config.NamespaceId, make(chan int)}

	return
}

func (client *Client) GetValues(keys []string) (map[string]string, error) {
	vars := make(map[string]string)
	for _, key := range keys {
		k := strings.TrimPrefix(key, "/")
		k = replacer.Replace(k)
		resp, err := client.client.GetConfig(vo.ConfigParam{
			DataId:  k,
			Group: client.group,
		})
		log.Info(fmt.Sprintf("key=%s, value=%s", key, resp))
		if err == nil {
			vars[key] = resp
		}
	}

	return vars, nil
}

func (client *Client) WatchPrefix(prefix string, keys []string, waitIndex uint64, stopChan chan bool) (uint64, error) {
	// return something > 0 to trigger a key retrieval from the store
	if waitIndex == 0 {
		for _, key := range keys {
			k := strings.TrimPrefix(key, "/")
			k = replacer.Replace(k)

			err := client.client.ListenConfig(vo.ConfigParam{
				DataId:  k,
				Group: client.group,
				OnChange: func(namespace, group, dataId, data string) {
					log.Info(fmt.Sprintf("config namespace=%s, dataId=%s, group=%s has changed", namespace, dataId, group))
					client.channel <- 1
				},
			})
			if err != nil {
				return 0,err
			}
		}

		return 1, nil
	}

	select {
		case <- client.channel:
			return waitIndex,nil
	}

	return waitIndex, nil
}
