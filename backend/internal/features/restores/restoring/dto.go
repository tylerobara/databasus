package restoring

import (
	"time"

	"github.com/google/uuid"

	"databasus-backend/internal/features/databases/databases/mariadb"
	"databasus-backend/internal/features/databases/databases/mongodb"
	"databasus-backend/internal/features/databases/databases/mysql"
	"databasus-backend/internal/features/databases/databases/postgresql"
)

type RestoreDatabaseCache struct {
	PostgresqlDatabase *postgresql.PostgresqlDatabase `json:"postgresqlDatabase,omitempty"`
	MysqlDatabase      *mysql.MysqlDatabase           `json:"mysqlDatabase,omitempty"`
	MariadbDatabase    *mariadb.MariadbDatabase       `json:"mariadbDatabase,omitempty"`
	MongodbDatabase    *mongodb.MongodbDatabase       `json:"mongodbDatabase,omitempty"`
}

type RestoreToNodeRelation struct {
	NodeID     uuid.UUID   `json:"nodeId"`
	RestoreIDs []uuid.UUID `json:"restoreIds"`
}

type RestoreNode struct {
	ID            uuid.UUID `json:"id"`
	ThroughputMBs int       `json:"throughputMBs"`
	LastHeartbeat time.Time `json:"lastHeartbeat"`
}

type RestoreNodeStats struct {
	ID             uuid.UUID `json:"id"`
	ActiveRestores int       `json:"activeRestores"`
}

type RestoreSubmitMessage struct {
	NodeID         uuid.UUID `json:"nodeId"`
	RestoreID      uuid.UUID `json:"restoreId"`
	IsCallNotifier bool      `json:"isCallNotifier"`
}

type RestoreCompletionMessage struct {
	NodeID    uuid.UUID `json:"nodeId"`
	RestoreID uuid.UUID `json:"restoreId"`
}
