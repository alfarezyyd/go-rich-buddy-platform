package injector

import (
	"context"
	"time"

	"go-rich-buddy-platform/client"
	"go-rich-buddy-platform/config"
	"go-rich-buddy-platform/internal/agent"
	"go-rich-buddy-platform/internal/cache"
	"go-rich-buddy-platform/internal/memory"
	"go-rich-buddy-platform/internal/memory/retention"
	"go-rich-buddy-platform/internal/radar"
	"go-rich-buddy-platform/internal/tool"
	"go-rich-buddy-platform/internal/tool/sectors"
	"go-rich-buddy-platform/internal/user"
	validatorService "go-rich-buddy-platform/internal/validator"
	whatsappGateway "go-rich-buddy-platform/internal/whatsapp"
	whatsappSession "go-rich-buddy-platform/internal/whatsapp_session"
	"go-rich-buddy-platform/pkg/exception"
	pkgi18n "go-rich-buddy-platform/pkg/i18n"
	"go-rich-buddy-platform/pkg/middleware"
	"go-rich-buddy-platform/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	universalTranslator "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
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

func NewI18nBundle() *pkgi18n.Bundle {
	return pkgi18n.NewBundle()
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
	NewI18nBundle,
))

var ApplicationRoutesModule = fx.Module("applicationRoutes",
	fx.Provide(
		routes.NewPublicRoutes,
		routes.NewAuthenticationRoutes,
		routes.NewProtectedRoutes,
		routes.NewAgentRoutes,
		routes.NewWhatsappRoutes,
		routes.NewRadarRoutes,
		func(
			ginEngine *gin.Engine,
			publicRoutes *routes.PublicRoutes,
			authenticationRoutes *routes.AuthenticationRoutes,
			protectedRoutes *routes.ProtectedRoutes,
			agentRoutes *routes.AgentRoutes,
			whatsappRoutes *routes.WhatsappRoutes,
			radarRoutes *routes.RadarRoutes,
		) *routes.ApplicationRoutes {
			return routes.NewApplicationRoutes(ginEngine, publicRoutes, authenticationRoutes, protectedRoutes, agentRoutes, whatsappRoutes, radarRoutes)
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

func NewSectorsClient(restyModule *config.RestyModule) radar.SectorsClient {
	return radar.NewSectorsClient(restyModule.GetRestySectors())
}

var RadarModule = fx.Module("radarFeature",
	fx.Provide(
		NewSectorsClient,
		fx.Annotate(radar.NewRepository, fx.As(new(radar.Repository))),
		fx.Annotate(radar.NewService, fx.As(new(radar.Service))),
		fx.Annotate(radar.NewHandler, fx.As(new(radar.Controller))),
	),
	// Start the pipeline scheduler on application startup
	fx.Invoke(func(lc fx.Lifecycle, radarService radar.Service, db *gorm.DB) {
		scheduler := radar.NewScheduler(radarService, db, nil) // broadcastFunc wired separately via WhatsApp module
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				logrus.Info("Starting Radar pipeline scheduler")
				scheduler.Start()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				logrus.Info("Stopping Radar pipeline scheduler")
				scheduler.Stop()
				return nil
			},
		})
	}),
)

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

var WhatsappGatewayModule = fx.Module("whatsappGatewayFeature",
	fx.Provide(fx.Annotate(whatsappGateway.NewService, fx.As(new(whatsappGateway.Service)))),
	fx.Provide(whatsappGateway.NewGatewayHandler), // Not an interface for handler
)

var WhatsappSessionModule = fx.Module("whatsappSessionFeature",
	fx.Provide(fx.Annotate(whatsappSession.NewRepository, fx.As(new(whatsappSession.SessionRepository)))),
)

// NewCacheService creates the layered cache backed by Redis.
func NewCacheService(redisInstance *config.RedisInstance) cache.Cache {
	return cache.NewLayeredCache(redisInstance.RedisClient)
}

// NewMemoryService creates the memory orchestration service.
func NewMemoryService(dbConnection *gorm.DB, agentClient client.AgentClient, viperConfig *viper.Viper) memory.Service {
	extractorModel := viperConfig.GetString("MEMORY_EXTRACTOR_MODEL")
	summarizerModel := viperConfig.GetString("MEMORY_SUMMARIZER_MODEL")
	return memory.NewService(dbConnection, agentClient, extractorModel, summarizerModel)
}

var CacheModule = fx.Module("cacheFeature",
	fx.Provide(fx.Annotate(NewCacheService, fx.As(new(cache.Cache)))),
)

var MemoryModule = fx.Module("memoryFeature",
	fx.Provide(fx.Annotate(NewMemoryService, fx.As(new(memory.Service)))),
	// Schedule the daily retention job.
	fx.Invoke(func(lc fx.Lifecycle, dbConnection *gorm.DB) {
		retentionJob := retention.NewJob(dbConnection)
		cronScheduler := cron.New()
		_, _ = cronScheduler.AddFunc("30 3 * * *", func() {
			retentionJob.Run(context.Background())
		})
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				logrus.Info("Starting memory retention scheduler")
				cronScheduler.Start()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				logrus.Info("Stopping memory retention scheduler")
				cronScheduler.Stop()
				return nil
			},
		})
	}),
)
