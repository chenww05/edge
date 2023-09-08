package halo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/example/minibox/configs"
	"github.com/example/minibox/mock"
	"github.com/example/turing-common/log"
)

func getMockHaloApi(box *mock.MockBox, db *mock.MockDBClient) *HaloAPI {
	return &HaloAPI{
		Box:    box,
		DB:     db,
		Logger: log.Logger("halo_api"),
	}
}

type mockErrReader struct{}

func (m mockErrReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("mock error")
}

func TestRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	db := mock.NewMockDBClient(ctrl)
	box := mock.NewMockBox(ctrl)

	injector := configs.GetInjector()
	injector.Map(box)
	injector.Map(db)
	t.Run("register success", func(t *testing.T) {
		t.Parallel()
		Register(configs.GetInjector(), gin.Default())
	})
}

func TestHaloAPI_BaseURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)
	u := getMockHaloApi(box, db)
	url := u.BaseURL()
	assert.Equal(t, "halo", url)
}

func TestHaloAPI_Middlewares(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)
	u := getMockHaloApi(box, db)
	mds := u.Middlewares()
	assert.EqualValues(t, []gin.HandlerFunc{}, mds)
}

func TestHaloAPI_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)

	u := getMockHaloApi(box, db)
	router := gin.Default().RouterGroup
	group := router.Group(u.BaseURL())
	u.Register(group)
}

func TestHaloAPI_UploadEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)

	h := getMockHaloApi(box, db)

	g := gin.Default()
	router := g.RouterGroup
	group := router.Group(h.BaseURL())
	h.Register(group)

	w := httptest.NewRecorder()

	t.Run("GetRawData error", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/halo/event", mockErrReader{})
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("unexpected end of json input", func(t *testing.T) {
		r := bytes.NewReader([]byte{})
		req, _ := http.NewRequest("POST", "/halo/event", r)
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("unknown event type", func(t *testing.T) {
		buff, _ := json.Marshal(HaloEventNotification{EventType: "test"})
		req, _ := http.NewRequest("POST", "/halo/event", bytes.NewReader(buff))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})
}

func TestHaloAPI_UploadStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)

	h := getMockHaloApi(box, db)

	g := gin.Default()
	router := g.RouterGroup
	group := router.Group(h.BaseURL())
	h.Register(group)

	w := httptest.NewRecorder()

	t.Run("GetRawData error", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/halo/heartbeat", mockErrReader{})
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("unexpected end of json input", func(t *testing.T) {
		r := bytes.NewReader([]byte{})
		req, _ := http.NewRequest("GET", "/halo/heartbeat", r)
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})
}
