package box

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/configs"
	"github.com/turingvideo/minibox/mock"
)

func S3StubServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		xml.NewEncoder(w).Encode(s3Response{})
	})

	return httptest.NewServer(mux)
}

func S3GwServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(gwResponse{})
	})

	return httptest.NewServer(mux)
}

func Test_baseBox_newUploadRequest(t *testing.T) {
	cfg := configs.NewEmptyConfig()
	type args struct {
		url      string
		field    string
		filename string
		params   map[string]string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "success",
			args: args{
				url:      "",
				field:    "",
				filename: "./s3.go",
				params:   nil,
			},
		},
		{
			name: "no file",
			args: args{
				url:      "",
				field:    "",
				filename: "",
				params:   nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			box := New(&cfg, configs.BoxInfo{}, nil)
			b := box.(*baseBox)
			_, err := b.newUploadRequest(tt.args.url, tt.args.field, tt.args.filename, tt.args.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("newUploadRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_UploadS3ByTokenName(t *testing.T) {
	t.Run("stat file error", func(t *testing.T) {
		cfg := configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		_, err := box.UploadS3ByTokenName(1, "filename", 0, 0, "mp4", TokenNameCameraVideo)
		assert.NotNil(t, err)
	})

	t.Run("invalid content type", func(t *testing.T) {
		cfg := configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		_, err := box.UploadS3ByTokenName(1, "./s3.go", 0, 0, "video", TokenNameCameraVideo)
		assert.NotNil(t, err)
	})

	t.Run("uploadMediaFileToS3 error", func(t *testing.T) {
		cfg := configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		_, err := box.UploadS3ByTokenName(1, "./s3.go", 0, 0, "ts", TokenNameCameraVideo)
		assert.NotNil(t, err)
	})

	t.Run("error is nil", func(t *testing.T) {
		server := S3StubServer()
		defer server.Close()
		cfg := configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		b := box.(*baseBox)
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client := mock.NewMockClient(ctrl)
		tk := &cloud.Token{Url: server.URL}
		client.EXPECT().GetTokenByCamera(gomock.Any(), gomock.Eq(TokenNameCameraVideo)).Return(tk, nil).AnyTimes()
		b.apiClient = client
		_, err := b.UploadS3ByTokenName(1, "./s3.go", 0, 0, "mp4", TokenNameCameraVideo)
		assert.Nil(t, err)
	})
}

func TestUploadMediaFileToS3(t *testing.T) {
	t.Run("GetToken error", func(t *testing.T) {
		cfg := configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		b := box.(*baseBox)
		_, err := b.uploadMediaFileToS3(1, "./s3.go", "video/mp4", TokenNameCameraVideo)
		assert.NotNil(t, err)
	})

	t.Run("error is nil, get by camera", func(t *testing.T) {
		server := S3StubServer()
		defer server.Close()
		cfg := configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		b := box.(*baseBox)
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client := mock.NewMockClient(ctrl)
		tk := &cloud.Token{Url: server.URL}
		client.EXPECT().GetTokenByCamera(gomock.Any(), gomock.Eq(TokenNameCameraVideo)).Return(tk, nil).AnyTimes()
		b.apiClient = client
		_, err := b.uploadMediaFileToS3(1, "./s3.go", "video/mp4", TokenNameCameraVideo)
		assert.Nil(t, err)
	})

	t.Run("error is nil, get by box", func(t *testing.T) {
		server := S3StubServer()
		defer server.Close()
		cfg := configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		b := box.(*baseBox)
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client := mock.NewMockClient(ctrl)
		tk := &cloud.Token{Url: server.URL}
		client.EXPECT().GetTokenByBox(gomock.Eq(TokenNameCameraVideo)).Return(tk, nil).AnyTimes()
		b.apiClient = client
		_, err := b.uploadMediaFileToS3(0, "./s3.go", "video/mp4", TokenNameCameraVideo)
		assert.Nil(t, err)
	})

	t.Run("EnableGateway true,error is nil", func(t *testing.T) {
		server := S3GwServer()
		defer server.Close()
		cfg := configs.NewEmptyConfig()
		cfg.SetUploadConfig(&configs.UploadConfig{EnableGateway: true, GatewayUploadUrl: server.URL})
		box := New(&cfg, configs.BoxInfo{}, nil)
		b := box.(*baseBox)
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client := mock.NewMockClient(ctrl)
		tk := &cloud.Token{Url: server.URL}
		client.EXPECT().GetTokenByCamera(gomock.Any(), gomock.Eq(TokenNameCameraVideo)).Return(tk, nil).AnyTimes()
		b.apiClient = client
		_, err := b.uploadMediaFileToS3(1, "./s3.go", "video/mp4", TokenNameCameraVideo)
		assert.Nil(t, err)
	})
}
