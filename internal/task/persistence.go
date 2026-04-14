package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PersistenceConfig 持久化配置
type PersistenceConfig struct {
	Enabled       bool   `yaml:"enabled"`
	FilePath      string `yaml:"file_path"`
	RestoreOnLoad bool   `yaml:"restore_on_load"`
	SaveInterval  string `yaml:"save_interval"`
}

// 默认配置
var DefaultPersistenceConfig = PersistenceConfig{
	Enabled:       false,
	FilePath:      "/var/lib/ooc-server/tasks.json",
	RestoreOnLoad: true,
	SaveInterval:  "5m",
}

// PersistenceManager 持久化管理器
type PersistenceManager struct {
	store      TaskStore
	config     PersistenceConfig
	filePath   string
	mutex      sync.RWMutex
	saveTicker *time.Ticker
	running    bool
}

// NewPersistenceManager 创建持久化管理器
func NewPersistenceManager(store TaskStore, config PersistenceConfig) *PersistenceManager {
	if config.FilePath == "" {
		config.FilePath = DefaultPersistenceConfig.FilePath
	}

	return &PersistenceManager{
		store:    store,
		config:   config,
		filePath: config.FilePath,
		running:  false,
	}
}

// Start 启动持久化管理器
func (pm *PersistenceManager) Start() error {
	if !pm.config.Enabled {
		return nil
	}

	// 创建目录
	dir := filepath.Dir(pm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// 恢复任务（如果启用）
	if pm.config.RestoreOnLoad {
		if err := pm.restore(); err != nil {
			return fmt.Errorf("failed to restore tasks: %w", err)
		}
	}

	// 启动定时保存
	duration, err := time.ParseDuration(pm.config.SaveInterval)
	if err != nil {
		// 使用默认值
		duration = 5 * time.Minute
	}

	// 最小间隔 1 分钟
	if duration < time.Minute {
		duration = time.Minute
	}

	pm.saveTicker = time.NewTicker(duration)
	go pm.saveLoop()

	pm.running = true
	return nil
}

// Stop 停止持久化管理器
func (pm *PersistenceManager) Stop() {
	if !pm.running {
		return
	}

	if pm.saveTicker != nil {
		pm.saveTicker.Stop()
	}

	// 立即保存一次
	if err := pm.save(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save tasks on stop: %v\n", err)
	}

	pm.running = false
}

// save 保存任务状态到文件
func (pm *PersistenceManager) save() error {
	// 获取所有任务
	tasks := pm.store.GetAll()

	// 序列化为 JSON
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(pm.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tasks file: %w", err)
	}

	return nil
}

// saveLoop 定时保存任务
func (pm *PersistenceManager) saveLoop() {
	for range pm.saveTicker.C {
		if err := pm.save(); err != nil {
			// 记录错误但不终止
			fmt.Fprintf(os.Stderr, "Failed to save tasks: %v\n", err)
		}
	}
}

// restore 从文件恢复任务
func (pm *PersistenceManager) restore() error {
	data, err := os.ReadFile(pm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在是正常情况
			return nil
		}
		return fmt.Errorf("failed to read tasks file: %w", err)
	}

	// 反序列化
	var tasks map[string]*Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return fmt.Errorf("failed to unmarshal tasks: %w", err)
	}

	// 将恢复的任务添加到内存存储
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	for _, task := range tasks {
		if err := pm.store.Save(task); err != nil {
			// 记录错误但继续恢复其他任务
			fmt.Fprintf(os.Stderr, "Failed to save restored task %s: %v\n", task.ID, err)
		}
	}

	return nil
}
