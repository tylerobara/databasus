package notifiers

import (
	"log/slog"

	"github.com/google/uuid"

	"databasus-backend/internal/util/encryption"
)

type NotificationSender interface {
	Send(
		encryptor encryption.FieldEncryptor,
		logger *slog.Logger,
		heading string,
		message string,
	) error

	Validate(encryptor encryption.FieldEncryptor) error

	HideSensitiveData()

	EncryptSensitiveData(encryptor encryption.FieldEncryptor) error
}

type NotifierDatabaseCounter interface {
	GetNotifierAttachedDatabasesIDs(notifierID uuid.UUID) ([]uuid.UUID, error)
}
