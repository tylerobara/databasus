package backups_download

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"databasus-backend/internal/storage"
)

type DownloadTokenRepository struct{}

func (r *DownloadTokenRepository) Create(token *DownloadToken) error {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}
	return storage.GetDb().Create(token).Error
}

func (r *DownloadTokenRepository) FindByToken(token string) (*DownloadToken, error) {
	var downloadToken DownloadToken

	err := storage.GetDb().
		Where("token = ?", token).
		First(&downloadToken).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &downloadToken, nil
}

func (r *DownloadTokenRepository) Update(token *DownloadToken) error {
	return storage.GetDb().Save(token).Error
}

func (r *DownloadTokenRepository) DeleteExpired(before time.Time) error {
	return storage.GetDb().
		Where("expires_at < ?", before).
		Delete(&DownloadToken{}).Error
}

func GenerateSecureToken() string {
	b := make([]byte, 32)

	if _, err := rand.Read(b); err != nil {
		panic("failed to generate secure random token: " + err.Error())
	}

	return base64.URLEncoding.EncodeToString(b)
}
