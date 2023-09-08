package uniview

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	box2 "github.com/example/minibox/box"
	"github.com/example/minibox/mock"
	"github.com/example/minibox/utils"
)

func TestUniviewAPI_uploadEventMediaToCloud(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	dbCli := mock.NewMockDBClient(ctrl)
	t.Run("success upload event media to cloud", func(t *testing.T) {
		box.EXPECT().UploadS3ByTokenName(1, "test_video_path", 0, 0, videoType, box2.TokenNameCameraEvent).Return(nil, errors.New("net timeout")).Times(1)
		box.EXPECT().UploadS3ByTokenName(1, "test_video_path", 0, 0, videoType, box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
		box.EXPECT().UploadEventMedia("test_event_id", gomock.Any()).Return(nil)
		u := getMockUniviewApi(box, dbCli)
		s3File, err := u.uploadEventMediaToCloud(1, "test_event_id", "test_video_path", 10, 20)
		assert.Nil(t, err)
		assert.NotNil(t, s3File)
	})
}
