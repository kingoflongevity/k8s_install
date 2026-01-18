package log

import (
	"database/sql"
	"fmt"
	"sync"
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

// LogSubscription 日志订阅结构体
type LogSubscription struct {
	Ch <-chan LogEntry
	Id string
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
	// SubscribeLogs 订阅日志事件
	SubscribeLogs() LogSubscription
	// UnsubscribeLogs 取消订阅日志事件
	UnsubscribeLogs(sub LogSubscription)
}

// SqliteLogManager SQLite日志管理器
type SqliteLogManager struct {
	DB                  *sql.DB
	broadcastChan       chan LogEntry
	subscribers         map[string]chan LogEntry
	mutex               sync.RWMutex
	broadcastChanClosed bool
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

	// 初始化广播通道和订阅者映射
	broadcastChan := make(chan LogEntry, 100)

	// 启动广播协程
	manager := &SqliteLogManager{
		DB:                  db,
		broadcastChan:       broadcastChan,
		subscribers:         make(map[string]chan LogEntry),
		broadcastChanClosed: false,
	}

	// 启动广播协程
	go manager.broadcastLogs()

	return manager, nil
}

// broadcastLogs 广播日志到所有订阅者
func (m *SqliteLogManager) broadcastLogs() {
	for logEntry := range m.broadcastChan {
		m.mutex.RLock()
		// 创建订阅者列表的副本，避免在遍历过程中修改
		subscribers := make([]chan LogEntry, 0, len(m.subscribers))
		for _, ch := range m.subscribers {
			subscribers = append(subscribers, ch)
		}
		m.mutex.RUnlock()

		// 发送日志到所有订阅者
		for _, ch := range subscribers {
			select {
			case ch <- logEntry:
				// 日志发送成功
			default:
				// 通道已满，跳过此日志以避免阻塞
				// 可以考虑关闭这个通道，因为订阅者可能已经断开连接
				m.mutex.Lock()
				// 遍历所有订阅者，寻找对应的通道并删除
				for id, subCh := range m.subscribers {
					if subCh == ch {
						close(subCh)
						delete(m.subscribers, id)
						break
					}
				}
				m.mutex.Unlock()
			}
		}
	}

	// 广播通道关闭，关闭所有订阅者通道
	m.mutex.Lock()
	for id, ch := range m.subscribers {
		close(ch)
		delete(m.subscribers, id)
	}
	m.mutex.Unlock()
}

// SubscribeLogs 订阅日志事件
func (m *SqliteLogManager) SubscribeLogs() LogSubscription {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 创建一个带缓冲的通道，避免阻塞
	ch := make(chan LogEntry, 100)
	// 生成唯一ID
	id := fmt.Sprintf("sub_%d", time.Now().UnixNano())
	// 将通道存储到订阅者映射中
	m.subscribers[id] = ch
	// 返回订阅结构体
	return LogSubscription{
		Ch: ch,
		Id: id,
	}
}

// UnsubscribeLogs 取消订阅日志事件
func (m *SqliteLogManager) UnsubscribeLogs(sub LogSubscription) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查订阅ID是否存在
	if ch, exists := m.subscribers[sub.Id]; exists {
		// 关闭通道
		close(ch)
		// 从订阅者列表中移除
		delete(m.subscribers, sub.Id)
	}
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

	// 将日志发送到广播通道，由broadcastLogs协程处理
	select {
	case m.broadcastChan <- log:
		// 日志发送成功到广播通道
	default:
		// 广播通道已满，跳过此日志以避免阻塞
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
