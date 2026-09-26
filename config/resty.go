package config

import (
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"
)

type HttpTarget struct {
	Endpoint  string `json:"endpoint"`
	AuthToken string `json:"auth_token"`
}

type RestyConfig struct {
	RetryCount       int
	RetryWaitTime    time.Duration
	RetryMaxWaitTime time.Duration
	Timeout          time.Duration
	AgentRouter      HttpTarget
	Sectors          HttpTarget
}

type RestyModule struct {
	restyConfig          *RestyConfig
	restyAgentRouter     *resty.Client
	restyWhatsappGateway *resty.Client
	restySectors         *resty.Client
}

func NewRestyConfig(viperConfig *viper.Viper) *RestyConfig {
	agentRouterEndpoint := viperConfig.GetString("AGENT_BASE_URL")
	agentRouterAuthToken := viperConfig.GetString("AGENT_APIKEY")
	sectorsEndpoint := viperConfig.GetString("SECTORS_API_URL")
	sectorsAuthToken := viperConfig.GetString("SECTORS_API_KEY")

	return &RestyConfig{
		RetryCount:       3,
		RetryWaitTime:    100 * time.Millisecond,
		RetryMaxWaitTime: 2 * time.Second,
		Timeout:          30 * time.Second,
		AgentRouter: HttpTarget{
			Endpoint:  agentRouterEndpoint,
			AuthToken: agentRouterAuthToken,
		},
		Sectors: HttpTarget{
			Endpoint:  sectorsEndpoint,
			AuthToken: sectorsAuthToken,
		},
	}
}

func NewRestyModule(restyConfig *RestyConfig) *RestyModule {
	restyModule := &RestyModule{
		restyConfig: restyConfig,
	}
	restyModule.restyAgentRouter = restyModule.getBaseResty().
		SetBaseURL(restyConfig.AgentRouter.Endpoint).
		SetAuthToken(restyConfig.AgentRouter.AuthToken)

	restyModule.restyWhatsappGateway = restyModule.getBaseResty().
		SetBaseURL("http://localhost:3000").
		SetHeader("X-Device-Id", "Production Device")

	restyModule.restySectors = restyModule.getBaseResty().
		SetBaseURL(restyConfig.Sectors.Endpoint).
		SetHeader("Authorization", restyConfig.Sectors.AuthToken)

	return restyModule
}

func (restyModule *RestyModule) getBaseResty() *resty.Client {
	return resty.New().
		SetRetryCount(restyModule.restyConfig.RetryCount)
}

func (restyModule *RestyModule) GetRestyAgentRouter() *resty.Client {
	return restyModule.restyAgentRouter
}

func (restyModule *RestyModule) GetRestyWhatsappGateway() *resty.Client {
	return restyModule.restyWhatsappGateway
}

func (restyModule *RestyModule) GetRestySectors() *resty.Client {
	return restyModule.restySectors
}
