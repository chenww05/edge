package box

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/example/minibox/cloud"
)

func (b *baseBox) getTokenFromCache(name string) *cloud.Token {
	val, ok := b.tokenCache.Load(name)
	if !ok {
		b.logger.Error().Msg("Token not found in cache")
		return nil
	}
	tokenOrg, ok := val.(cloud.Token)
	if !ok {
		b.logger.Error().Msg("val not cloud.Token")
		return nil
	}
	var token *cloud.Token
	byt, _ := json.Marshal(tokenOrg)
	if err := json.Unmarshal(byt, &token); err != nil {
		b.logger.Error().Msg("Token Unmarshal Error")
		return nil
	}
	return token
}

func (b *baseBox) setTokenInCache(name string, token cloud.Token) {
	b.tokenCache.Store(name, token)
}

func (b *baseBox) delTokenInCache(name string) {
	b.tokenCache.Delete(name)
}

func (b *baseBox) GetTokenByBox(name string) (*cloud.Token, error) {
	b.logger.Debug().Msgf("GetTokenByBox, tokenName: %s", name)
	// Box level get token use the org expiration, use name as cacheKey is enough
	cacheKey := name
	token := b.getTokenFromCache(cacheKey)
	if token != nil {
		return token, nil
	}
	if b.apiClient == nil {
		return nil, ErrNoAPIClient
	}
	// lock this prevent dirty read
	b.mux.Lock()
	defer b.mux.Unlock()
	token = b.getTokenFromCache(cacheKey)
	if token != nil {
		return token, nil
	}
	token, err := b.apiClient.GetTokenByBox(name)
	if err != nil {
		return nil, err
	}
	b.setTokenInCache(cacheKey, *token)
	b.logger.Debug().Msgf("GetTokenByBox, token: %+v", *token)
	return token, nil
}

func (b *baseBox) formatCacheKey(cameraId int, name string) (string, error) {
	switch name {
	case TokenNameCameraEvent, TokenNameCameraVideo:
		return fmt.Sprintf("%s_%d_%d", name, cameraId, cloud.GetEventExpireDayByCameraID(cameraId)), nil
	case TokenNameCloudStorage:
		return fmt.Sprintf("%s_%d_%d", name, cameraId, cloud.GetCloudStorageExpireDayByCameraID(cameraId)), nil
	case TokenNameCameraSnap:
		// Snap expiration use the org level snap expiration, return name is enough, same to line:41
		return name, nil
	default:
		return "", errors.New("category is not support")
	}
}

func (b *baseBox) GetTokenByCamera(cameraId int, name string) (*cloud.Token, error) {
	b.logger.Debug().Msgf("GetTokenByCamera, cameraId: %d, tokenName: %s", cameraId, name)
	cacheKey, err := b.formatCacheKey(cameraId, name)
	if err != nil {
		return nil, err
	}
	token := b.getTokenFromCache(cacheKey)
	if token != nil {
		return token, nil
	}
	if b.apiClient == nil {
		return nil, ErrNoAPIClient
	}
	// lock this prevent dirty read
	b.mux.Lock()
	defer b.mux.Unlock()
	token = b.getTokenFromCache(cacheKey)
	if token != nil {
		return token, nil
	}
	token, err = b.apiClient.GetTokenByCamera(cameraId, name)
	if err != nil {
		return nil, err
	}
	b.setTokenInCache(cacheKey, *token)
	b.logger.Debug().Msgf("GetTokenByCamera, token: %+v", *token)
	return token, nil
}
