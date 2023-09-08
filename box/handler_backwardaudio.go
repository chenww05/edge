package box

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/mitchellh/mapstructure"
	"github.com/example/turing-common/websocket"

	"github.com/example/minibox/stream"
)

func (h *handler) startGeneralBackwardAudio(msg websocket.Message) ([]byte, error) {
	var bwar generalStartBackwardAudioReq
	err := mapstructure.Decode(msg.GetArgs(), &bwar)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	if err := bwar.Validate(); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	// backward audio is only allowed one audio input to camera.
	if h.getStreamManager().HasStream(bwar.StreamId) {
		return msg.ReplyMessage(websocket.Err{
			Code:           -1,
			DevelopMessage: ErrBackwardAudioOngoing.Error(),
			Message:        ErrBackwardAudioOngoing.Error(),
		}).Marshal(), nil
	}
	var outputUrl string
	inputUrl := bwar.OutputUri
	var streamType string
	audioEncodeType := AudioEncodeTypeG711U // default set to mulaw
	switch bwar.DeviceType {
	case AudioDTSpeaker:
		outputUrl, err = h.getBackwardSpeakerAudioUrl(bwar.DeviceStreamUri, bwar.DeviceUsername, bwar.DevicePassword)
		if err != nil {
			return msg.ReplyMessage(errors.New("invalid speaker url")).Marshal(), nil
		}
		streamType = string(stream.BackwardAudioSpeaker)
	default:
		return msg.ReplyMessage(fmt.Errorf("not supported deviceType %s", bwar.DeviceType)).Marshal(), nil
	}
	if outputUrl == "" {
		return msg.ReplyMessage(ErrNotBackwardAudioCamera).Marshal(), nil
	}
	outputUri, err := h.getStreamManager().StartStream(bwar.StreamId, streamType, inputUrl, outputUrl)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	h.log.Info().Msgf("start general backward audio pull stream from %s and send to %s,outputUrl %s", inputUrl, outputUri, outputUrl)
	return msg.ReplyMessage(backwardAudioResp{EncodeInfo: AudioEncodeInfo{
		EncodeType: audioEncodeType,
	}}).Marshal(), err
}

func (h *handler) stopGeneralBackwardAudio(msg websocket.Message) ([]byte, error) {
	var bwar generalBackwardAudioReq
	err := mapstructure.Decode(msg.GetArgs(), &bwar)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	mValidator := validator.New()
	if err = mValidator.Struct(bwar); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	// backward audio has not started.
	if !h.getStreamManager().HasStream(bwar.StreamId) {
		return msg.ReplyMessage(websocket.Err{
			Code:           -1,
			DevelopMessage: ErrBackwardAudioNotStarted.Error(),
			Message:        ErrBackwardAudioNotStarted.Error(),
		}).Marshal(), nil
	}

	// stop stream.
	err = h.getStreamManager().StopStream(bwar.StreamId)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	return msg.ReplyMessage(nil).Marshal(), nil
}

func (h *handler) getBackwardSpeakerAudioUrl(host, username, password string) (string, error) {
	if !strings.HasPrefix(host, "http") {
		host = "http://" + host
	}
	parsedUrl, err := url.Parse(host)
	if err != nil {
		return "", err
	}
	parsedUrl.User = url.UserPassword(username, password)
	return parsedUrl.String(), nil
}

func (h *handler) heartbeatGeneralBackwardAudio(msg websocket.Message) ([]byte, error) {
	var args generalBackwardAudioReq
	err := mapstructure.Decode(msg.GetArgs(), &args)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	mValidator := validator.New()
	if err = mValidator.Struct(args); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	// backward audio has not started.
	if !h.getStreamManager().HasStream(args.StreamId) {
		h.log.Info().Msgf("speaker stream_id %s not exist", args.StreamId)
		return msg.ReplyMessage(websocket.Err{
			Code:           -1,
			DevelopMessage: ErrBackwardAudioNotStarted.Error(),
			Message:        ErrBackwardAudioNotStarted.Error(),
		}).Marshal(), nil
	}

	// judge if stream is alive.
	if h.getStreamManager().IsStreamStopped(args.StreamId) {
		h.log.Info().Msgf("speaker stream_id %s status is stopped", args.StreamId)
		return msg.ReplyMessage(websocket.Err{
			Code:           -2,
			DevelopMessage: ErrBackwardAudioHasStopped.Error(),
			Message:        ErrBackwardAudioHasStopped.Error(),
		}).Marshal(), nil
	}
	h.log.Debug().Msgf("speaker stream_id %s status is alive", args.StreamId)
	return msg.ReplyMessage(nil).Marshal(), nil
}
