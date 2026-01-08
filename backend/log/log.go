package log

import (
	"database/sql"
	"fmt"
	"time"

	// 使用纯Go实现的SQLite驱动，不需要CGO
	_ "modernc.org/sqlite"
)

// LogEntry 日志条目
type LogEntry struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"nodeId"`
	NodeName  string    `json:"nodeName"`
	Operation string    `json:"operation"`
	Command   string    `json:"command"`
	Output    string    `json:"output"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// LogManager 日志管理器接口
type LogManager interface {
	// CreateLog 创建新日志
	CreateLog(log LogEntry) error
	// GetLogs 获取所有日志
	GetLogs() ([]LogEntry, error)
	// GetLogsByNode 获取指定节点的日志
	GetLogsByNode(nodeID string) ([]LogEntry, error)
	// ClearLogs 清除所有日志
	ClearLogs() error
}

// SqliteLogManager SQLite日志管理器
type SqliteLogManager struct {
	DB *sql.DB
}

// NewSqliteLogManager 创建新的SQLite日志管理器
func NewSqliteLogManager(db *sql.DB) (*SqliteLogManager, error) {
	// 创建日志表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS logs (
		id TEXT PRIMARY KEY,
		node_id TEXT NOT NULL,
		node_name TEXT NOT NULL,
		operation TEXT NOT NULL,
		command TEXT NOT NULL,
		output TEXT,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);
	`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		return nil, err
	}

	// 检查并添加updated_at列（如果不存在）
	var columnExists bool
	checkColumnSQL := `
	SELECT COUNT(*) FROM pragma_table_info('logs') WHERE name = 'updated_at';
	`
	err = db.QueryRow(checkColumnSQL).Scan(&columnExists)
	if err != nil {
		return nil, fmt.Errorf("failed to check updated_at column: %v", err)
	}

	if !columnExists {
		// 添加updated_at列
		addColumnSQL := `
		ALTER TABLE logs ADD COLUMN updated_at DATETIME;
		`
		_, err = db.Exec(addColumnSQL)
		if err != nil {
			return nil, fmt.Errorf("failed to add updated_at column: %v", err)
		}
	}

	return &SqliteLogManager{
		DB: db,
	}, nil
}

// CreateLog 创建新日志
func (m *SqliteLogManager) CreateLog(log LogEntry) error {
	// 确保UpdatedAt有值
	if log.UpdatedAt.IsZero() {
		log.UpdatedAt = log.CreatedAt
	}

	// 检查日志是否已存在，如果存在则更新，否则插入
	var count int
	err := m.DB.QueryRow("SELECT COUNT(*) FROM logs WHERE id = ?", log.ID).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		// 更新现有日志
		_, err = m.DB.Exec(
			"UPDATE logs SET node_id = ?, node_name = ?, operation = ?, command = ?, output = ?, status = ?, created_at = ?, updated_at = ? WHERE id = ?",
			log.NodeID, log.NodeName, log.Operation, log.Command, log.Output, log.Status, log.CreatedAt, log.UpdatedAt, log.ID,
		)
	} else {
		// 插入新日志
		_, err = m.DB.Exec(
			"INSERT INTO logs (id, node_id, node_name, operation, command, output, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			log.ID, log.NodeID, log.NodeName, log.Operation, log.Command, log.Output, log.Status, log.CreatedAt, log.UpdatedAt,
		)
	}
	return err
}

// GetLogs 获取所有日志
func (m *SqliteLogManager) GetLogs() ([]LogEntry, error) {
	rows, err := m.DB.Query("SELECT id, node_id, node_name, operation, command, output, status, created_at, updated_at FROM logs ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		var updatedAt sql.NullTime
		if err := rows.Scan(
			&log.ID, &log.NodeID, &log.NodeName, &log.Operation, &log.Command, &log.Output, &log.Status, &log.CreatedAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		if updatedAt.Valid {
			log.UpdatedAt = updatedAt.Time
		} else {
			log.UpdatedAt = log.CreatedAt
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// GetLogsByNode 获取指定节点的日志
func (m *SqliteLogManager) GetLogsByNode(nodeID string) ([]LogEntry, error) {
	rows, err := m.DB.Query(
		"SELECT id, node_id, node_name, operation, command, output, status, created_at, updated_at FROM logs WHERE node_id = ? ORDER BY created_at DESC",
		nodeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		var updatedAt sql.NullTime
		if err := rows.Scan(
			&log.ID, &log.NodeID, &log.NodeName, &log.Operation, &log.Command, &log.Output, &log.Status, &log.CreatedAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		if updatedAt.Valid {
			log.UpdatedAt = updatedAt.Time
		} else {
			log.UpdatedAt = log.CreatedAt
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// ClearLogs 清除所有日志
func (m *SqliteLogManager) ClearLogs() error {
	_, err := m.DB.Exec("DELETE FROM logs")
	return err
}
