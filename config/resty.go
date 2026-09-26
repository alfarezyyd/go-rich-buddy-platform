package config

import (
	"time"

	"github.com/go-resty/resty/v2"
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
	NineRouter       HttpTarget
	Sectors          HttpTarget
}

type RestyModule struct {
	restyConfig          *RestyConfig
	restyNineRouter      *resty.Client
	restyWhatsappGateway *resty.Client
	restySectors         *resty.Client
}

func NewRestyModule(restyConfig *RestyConfig) *RestyModule {
	restyModule := &RestyModule{
		restyConfig: restyConfig,
	}
	restyModule.restyNineRouter = restyModule.getBaseResty().
		SetBaseURL("http://localhost:20128/v1").
		SetAuthToken("sk-16919f52ba2b8bd0-0ydrw1-fd953f4b")

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

func (restyModule *RestyModule) GetRestyNineRouter() *resty.Client {
	return restyModule.restyNineRouter
}

func (restyModule *RestyModule) GetRestyWhatsappGateway() *resty.Client {
	return restyModule.restyWhatsappGateway
}

func (restyModule *RestyModule) GetRestySectors() *resty.Client {
	return restyModule.restySectors
}
