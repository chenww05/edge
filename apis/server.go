package apis

import (
	"fmt"

	"github.com/codegangsta/inject"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	http2 "github.com/turingvideo/turing-common/http"

	"github.com/turingvideo/minibox/apis/guardian"
	"github.com/turingvideo/minibox/apis/halo"
	"github.com/turingvideo/minibox/apis/metrics"
	"github.com/turingvideo/minibox/apis/uniview"
	"github.com/turingvideo/minibox/configs"
)

func newEngine(injector inject.Injector) (*gin.Engine, error) {
	engine := gin.Default()
	// TODO(nick): change this to a cors middleware
	h := cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type", "Authorization"},
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTION", "HEAD"},
		AllowWebSockets:  true,
		AllowCredentials: true,
	})
	engine.Use(h)
	injector.Map(engine)

	initFuncs := []interface{}{
		uniview.Register,
		metrics.Register,
		guardian.Register,
		halo.Register,
		RegisterDumpAPI,
	}

	for _, f := range initFuncs {
		_, err := injector.Invoke(f)
		if err != nil {
			return nil, err
		}
	}
	engine.Use(http2.PromMiddleware(nil))
	return engine, nil
}

func Run(injector inject.Injector, cfg configs.Config) error {
	engine, err := newEngine(injector)
	if err != nil {
		return err
	}
	if cfg.GetEnablePprof() {
		pprof.Register(engine)
	}

	port := cfg.GetAPIServicePort()
	return engine.Run(fmt.Sprintf(":%d", port))
}
