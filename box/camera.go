package box

import (
	"github.com/example/minibox/camera"
	"github.com/example/minibox/camera/base"
)

func (b *baseBox) GetCamera(cameraID int) (base.Camera, error) {
	return b.camGroup.GetCamera(cameraID)
}

func (b *baseBox) AddCamera(c base.Camera) {
	err := b.camGroup.AddCamera(c)
	if err != nil {
		b.logger.Error().Err(err).Msgf("Failed to add camera to set")
	}

	cameraId := c.GetID()
	b.logger.Info().Str("camera_sn", c.GetSN()).Int("camera_id", cameraId).Msg("added camera to memory")

	if cameraId < 1 && b.apiClient != nil && !b.config.GetDisableCloud() {
		sn := c.GetSN()
		brand := c.GetBrand()
		ip := c.GetIP()
		cloudCamera, err := b.apiClient.AddCamera(sn, string(brand), ip)
		if err != nil {
			b.logger.Error().Msgf("failed to add camera to cloud: %s", err)
			return
		}

		b.logger.Info().Str("camera_sn", cloudCamera.SN).Int("camera_id", cloudCamera.ID).Msg("added camera to cloud")
		c.UpdateCamera(sn, cloudCamera.ID)
		b.logger.Info().Str("camera_sn", cloudCamera.SN).Int("camera_id", cloudCamera.ID).Msg("updated camera id in memory")
	}
}

func (b *baseBox) GetCameraBySN(sn string) (base.Camera, error) {
	return b.camGroup.GetCameraBySN(sn)
}

func (b *baseBox) GetCamGroup() camera.CamGroup {
	return b.camGroup
}
