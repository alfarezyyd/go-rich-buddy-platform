package injector

import (
	"go-rich-buddy-platform/client"
	"go-rich-buddy-platform/config"
	"go-rich-buddy-platform/internal/agent"
	"go-rich-buddy-platform/internal/tool"
	"go-rich-buddy-platform/internal/tool/sectors"
	"go-rich-buddy-platform/internal/user"
	validatorService "go-rich-buddy-platform/internal/validator"
	"go-rich-buddy-platform/pkg/exception"
	"go-rich-buddy-platform/pkg/middleware"
	"go-rich-buddy-platform/routes"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	universalTranslator "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

func NewDatabaseConnection(databaseCredentials *config.DatabaseCredentials) *gorm.DB {
	databaseConnectionInstance := config.NewDatabaseConnection(databaseCredentials)
	return databaseConnectionInstance.GetDatabaseConnection()
}

func NewRedisInstance(redisConfig config.RedisConfig) *config.RedisInstance {
	redisInstance, err := config.InitRedisInstance(redisConfig)
	if err != nil {
		panic(err)
	}
	return redisInstance
}

func InitRedisConfig(viperConfig *viper.Viper) config.RedisConfig {
	redisHostName, redisPassword, parsedRedisDatabaseIndex := config.LoadRedisConfigFromEnvironment(viperConfig)
	return config.NewRedisConfig(redisHostName, redisPassword, int(parsedRedisDatabaseIndex))
}

func NewValidator(gormDatabase *gorm.DB) (*validator.Validate, universalTranslator.Translator) {
	return config.InitializeValidator(gormDatabase)
}

func NewViperConfig() *viper.Viper {
	viperConfig := viper.New()
	viperConfig.SetConfigFile(".env")
	viperConfig.AddConfigPath(".")
	viperConfig.AutomaticEnv()
	if err := viperConfig.ReadInConfig(); err != nil {
		panic(err)
	}
	return viperConfig
}

func NewDatabaseCredentials(viperConfig *viper.Viper) *config.DatabaseCredentials {
	return &config.DatabaseCredentials{
		DatabaseHost:     viperConfig.GetString("DATABASE_HOST"),
		DatabasePort:     viperConfig.GetString("DATABASE_PORT"),
		DatabaseName:     viperConfig.GetString("DATABASE_NAME"),
		DatabasePassword: viperConfig.GetString("DATABASE_PASSWORD"),
		DatabaseUsername: viperConfig.GetString("DATABASE_USERNAME"),
	}
}

func NewGinEngine() (*gin.Engine, *gin.RouterGroup) {
	gin.SetMode(gin.DebugMode)
	ginEngine := gin.Default()
	ginEngine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"*"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	ginEngine.Use(gin.Recovery())
	ginEngine.Use(exception.Interceptor())
	ginEngineRoot := ginEngine.Group("/")
	ginEngineRoot.Use(middleware.RequestMetaMiddleware())

	return ginEngine, ginEngineRoot
}

var CoreModule = fx.Module("coreModule", fx.Provide(
	NewViperConfig,
	NewDatabaseCredentials,
	NewDatabaseConnection,
	NewValidator,
	NewGinEngine,
	InitRedisConfig,
	NewRedisInstance,
	config.NewRestyConfig,
	config.NewRestyModule,
))

var ApplicationRoutesModule = fx.Module("applicationRoutes",
	fx.Provide(
		routes.NewPublicRoutes,
		routes.NewAuthenticationRoutes,
		routes.NewProtectedRoutes,
		routes.NewAgentRoutes,
		func(
			ginEngine *gin.Engine,
			publicRoutes *routes.PublicRoutes,
			authenticationRoutes *routes.AuthenticationRoutes,
			protectedRoutes *routes.ProtectedRoutes,
			agentRoutes *routes.AgentRoutes,
		) *routes.ApplicationRoutes {
			return routes.NewApplicationRoutes(ginEngine, publicRoutes, authenticationRoutes, protectedRoutes, agentRoutes)
		},
	),
	fx.Invoke(func(applicationRoutes *routes.ApplicationRoutes) {
		applicationRoutes.Setup()
	}),
)

var UserModule = fx.Module("userFeature",
	fx.Provide(fx.Annotate(user.NewRepository, fx.As(new(user.Repository)))),
	fx.Provide(fx.Annotate(user.NewService, fx.As(new(user.Service)))),
	fx.Provide(fx.Annotate(user.NewHandler, fx.As(new(user.Controller)))),
)

var ValidatorModule = fx.Module("validatorFeature",
	fx.Provide(fx.Annotate(validatorService.NewService, fx.As(new(validatorService.Service)))),
)

func NewToolRegistry(restyModule *config.RestyModule) tool.Registry {
	toolRegistry := tool.NewRegistry()
	toolRegistry.RegisterAll(sectors.ProvideSectorsTools(restyModule.GetRestySectors())...)
	return toolRegistry
}

func NewAgentClient(viperConfig *viper.Viper) client.AgentClient {
	baseURL := viperConfig.GetString("AGENT_BASE_URL")
	apiKey := viperConfig.GetString("AGENT_APIKEY")
	return client.NewAgentClient(baseURL, apiKey)
}

var ToolModule = fx.Module("toolFeature",
	fx.Provide(
		NewToolRegistry,
	),
)

var AgentModule = fx.Module("agentFeature",
	fx.Provide(
		NewAgentClient,
		fx.Annotate(agent.NewClassifier, fx.As(new(agent.Classifier))),
		fx.Annotate(agent.NewService, fx.As(new(agent.Service))),
		fx.Annotate(agent.NewHandler, fx.As(new(agent.Controller))),
	),
)
