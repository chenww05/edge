package main

import (
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/example/turing-common/env"
	"github.com/example/turing-common/log"

	"github.com/example/minibox/apis"
	"github.com/example/minibox/apis/ppl_2"
	"github.com/example/minibox/box"
	"github.com/example/minibox/configs"
	"github.com/example/minibox/db"
	"github.com/example/minibox/discover"
	"github.com/example/minibox/discover/arp"
	"github.com/example/minibox/engine"
	"github.com/example/minibox/monitoring"
	"github.com/example/minibox/nvr/uniview"
	pemUtil "github.com/example/minibox/pem"
	"github.com/example/minibox/scheduler"
)

var logger zerolog.Logger
var boxVersion string

func init() {
	logger = log.Logger("main")
}

func main() {
	injector := configs.GetInjector()

	configFile, ok := os.LookupEnv("CONFIG_FILE")
	if !ok {
		logger.Fatal().Msg("Please export CONFIG_FILE")
	}
	cfg, err := configs.LoadConfig(configFile)
	if err != nil {
		logger.Fatal().Msgf("Load config error: %s", err)
	}
	injector.Map(cfg)

	hostname, _ := os.Hostname()
	log.InitGlobalLogger(env.Config{Hostname: hostname}, cfg.Logging())
	scheduler.InitSchedulerLogger(&scheduler.Config{
		ClipLogFilePath: cfg.GetStreamConfig().ClipLog,
		LiveLogFilePath: cfg.GetStreamConfig().LiveLog,
	})

	boxFile, ok := os.LookupEnv("BOX_FILE")
	if !ok {
		logger.Fatal().Msg("Please export BOX_FILE")
	}

	boxInfo, err := configs.LoadBoxFile(boxFile)
	if err != nil {
		logger.Fatal().Msgf("Box file error: %s", err)
	}
	boxInfo.Type = cfg.GetBoxType()

	if _, err = pemUtil.NewRSAEncryptor(); err != nil {
		logger.Warn().Msgf("Failed to init rsa encryptor")
	}

	// starting monitoring
	if cfg.GetZenodReportEnable() {
		monitoring.Report(boxInfo.ID, boxVersion, cfg.GetReportURI())
	}

	d, err := db.NewDBClient(cfg, db.DBPath)
	if err != nil {
		logger.Fatal().Msgf("Failed to init db: %s", err)
	}
	go d.Cleanup()
	injector.Map(d)

	scheduler.InitScheduler(cfg.GetStreamConfig())

	eng := engine.NewEngine()
	injector.Map(eng)

	searcher := discover.NewSearcherProcessor()
	discover.Run(5*time.Second, searcher)
	// start nvr
	b := box.New(cfg, *boxInfo, d)
	univNvr := uniview.NewNVRManager(b, d)
	b.SetNVRManager(univNvr)
	b.SetSearcher(searcher)
	arpSearcher := arp.NewSearcherProcessor()
	arpSearcher.Init()
	b.SetArpSearcher(arpSearcher)
	injector.Map(b)
	b.Start() // wait success run

	ppl_2.InitPcService(b, cfg.GetPpl2Cfg())
	// TODO Inject db into it.
	atr := box.NewArchiveTaskRunner(b, d, cfg.GetCloudStorageConfig())
	go atr.HandleArchiveTasks()
	atr.TryRecover()

	err = apis.Run(injector, cfg)
	if err != nil {
		logger.Fatal().Msgf("Failed to run api server: %s", err)
	}
}
