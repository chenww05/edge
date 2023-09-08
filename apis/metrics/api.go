package metrics

import (
	"github.com/codegangsta/inject"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	http2 "github.com/turingvideo/turing-common/http"
	"github.com/turingvideo/turing-common/log"
)

type MetricsAPI struct {
	Logger zerolog.Logger
}

func Register(injector inject.Injector, router *gin.Engine) {
	logger := log.Logger("metrics")
	api := &MetricsAPI{Logger: logger}
	if err := injector.Apply(api); err != nil {
		logger.Fatal().Err(err).Msg("Failed to init metrics api.")
	}
	http2.RegisterGinGroupHandler(&router.RouterGroup, api)
}

func (m *MetricsAPI) BaseURL() string {
	return ""
}

func (m *MetricsAPI) Middlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{}
}

func (m *MetricsAPI) Register(group *gin.RouterGroup) {
	group.GET("metrics", http2.PromHandler(promhttp.Handler()))
}
