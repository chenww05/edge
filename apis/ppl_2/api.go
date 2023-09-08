package ppl_2

import (
	"fmt"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"

	"github.com/turingvideo/turing-common/log"

	"github.com/turingvideo/minibox/apis/structs"
	"github.com/turingvideo/minibox/box"
	"github.com/turingvideo/minibox/camera/ppl_2"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/configs"
	"github.com/turingvideo/minibox/utils"
)

const (
	pplTimeRangeMethod = "timeRange"
	minSnapDuration    = 30
	statusTimeout      = 25 * time.Second
	ppl2EventType      = "people_count"
	cornExpression     = "*/%d * * * *"
	minInterval        = 1
	maxInterval        = 10
)

var once sync.Once

type PcService struct {
	box box.Box
	log zerolog.Logger
	cfg configs.Ppl2Config
}

func InitPcService(b box.Box, config configs.Ppl2Config) {
	once.Do(func() {
		pcs := &PcService{
			box: b,
			log: log.Logger("sunell_ppl_service"),
			cfg: config,
		}

		if pcs.cfg.Interval <= 0 {
			pcs.cfg.Interval = minInterval
		} else if pcs.cfg.Interval > maxInterval {
			pcs.cfg.Interval = maxInterval
		}

		err := pcs.startReport()
		if err != nil {
			pcs.log.Fatal().Msgf("report job start error: %s", err.Error())
			return
		}

		go pcs.startSnapShot()
		go pcs.cameraStatusMaintain()
	})
}

//mock a heartbeat
func (ps *PcService) cameraStatusMaintain() {
	ticker := time.NewTicker(statusTimeout)
	for {
		select {
		case <-ticker.C:
			for _, cam := range ps.getCamera() {
				if err := cam.GetDeviceInfo(); err == nil {
					cam.Heartbeat(cam.GetIP(), "")
				}
			}
		}
	}
}

func (ps *PcService) startReport() error {

	job := cron.New()

	expression := fmt.Sprintf(cornExpression, ps.cfg.Interval)
	id, err := job.AddFunc(expression, func() {
		ps.reportAllPeopleCounting()
	})

	if err != nil {
		return err
	}

	job.Start()
	ps.log.Info().Msgf("job: %d start successfully", id)

	return nil
}
func (ps *PcService) getCamera() []ppl_2.Ppl2Camera {
	cms := make([]ppl_2.Ppl2Camera, 0)

	cameras := ps.box.GetCamGroup().AllCameras()
	for _, camera := range cameras {
		if camera.GetBrand() == utils.SunellPpl {
			if bcm, ok := camera.(ppl_2.Ppl2Camera); ok {
				cms = append(cms, bcm)
			}
		}
	}

	return cms
}

func (ps *PcService) reportAllPeopleCounting() {

	var before time.Duration
	before = time.Duration(ps.cfg.Interval) * time.Minute

	startTime, endTime := time.Now().Add(-before), time.Now()
	for _, camera := range ps.getCamera() {
		ps.do(camera, startTime, endTime)
	}
}

func (ps *PcService) do(pc ppl_2.Ppl2Camera, start, end time.Time) {
	data, err := pc.GetPeopleCounting()
	if err != nil {
		ps.log.Error().Msgf("failed to get result for camera: %d, err: %s", pc.GetID(), err.Error())
		return
	}

	if err := ps.uploadToCloud(data, pc, start, end); err != nil {
		for i := 0; i < 3; i++ {
			err = ps.uploadToCloud(data, pc, start, end)
			if err == nil {
				break
			}
			ps.log.Error().Msgf("upload event err: %s, retry: %d, camera: %d", err.Error(), i, pc.GetID())
		}
	}
}

func (ps *PcService) uploadToCloud(req *ppl_2.CountingData, pc ppl_2.Ppl2Camera, start, end time.Time) error {
	meta := &structs.MetaScanData{
		HumanCount: &structs.HumanCountInfo{
			Enter:  req.Enter,
			Exit:   req.Leave,
			Method: pplTimeRangeMethod,
		},
	}

	startedAt, endedAt := start.UTC(), end.UTC()

	_, err := ps.box.UploadPplEvent(pc.GetID(), meta, startedAt, endedAt, ppl2EventType)
	if err != nil {
		ps.log.Error().Err(err).Msg("failed to upload event to cloud, Event Lost!")
		return err
	}
	ps.log.Info().Msgf("ppl2 event pushed: cameraID: %d, meta: %#v, start: %v, end: %v", pc.GetID(), meta.HumanCount, start, end)
	return nil
}

func (ps *PcService) startSnapShot() {
	snapDuration := ps.box.GetConfig().GetCameraConfig().SnapDuration
	if snapDuration < minSnapDuration {
		snapDuration = minSnapDuration
	}
	ps.allCamerasSnapShot()
	ticker := time.NewTicker(time.Duration(snapDuration) * time.Second)
	for {
		select {
		case <-ticker.C:
			ps.allCamerasSnapShot()
		}
	}
}

func (ps *PcService) allCamerasSnapShot() {
	for _, camera := range ps.getCamera() {
		ps.snapShot(camera)
	}
}

func (ps *PcService) snapShot(pc ppl_2.Ppl2Camera) {
	body, err := pc.GetImage()
	if err != nil {
		ps.log.Error().Msgf("get image failed: %s", err.Error())
		return
	}

	err = ps.uploadSnapshot(pc.GetID(), body)
	if err != nil {
		ps.log.Error().Msgf("camera: %d, upload snapshot to s3 failed: %s", pc.GetID(), err.Error())
		return
	}
	ps.log.Info().Msgf("camera: %d, upload snapshot success", pc.GetID())
}

func (ps *PcService) uploadSnapshot(camID int, body *ppl_2.Image) error {
	filename := filepath.Join(ps.box.GetConfig().GetDataStoreDir(), fmt.Sprintf("cam_snap_%s.jpeg", time.Now().Format(time.RFC3339)))
	err := utils.WriteFile([]byte(body.Dist), filename)
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(filename); err != nil {
			ps.log.Error().Msgf("failed to delete temp image file %v", err)
		}
	}()

	s3File, err := ps.box.UploadS3ByTokenName(camID, filename, body.Height, body.Width, "jpeg", box.TokenNameCameraSnap)
	if err != nil {
		return err
	}

	return ps.box.UploadCameraSnapshot(&cloud.CamSnapShotReq{
		CameraID:             camID,
		Timestamp:            time.Now().UTC().Format(utils.CloudTimeLayout),
		SnapFile:             s3File,
		SnapType:             "view",
		ShouldUpdateSnapshot: true,
	})
}
