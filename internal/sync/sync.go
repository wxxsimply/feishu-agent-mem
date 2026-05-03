package sync

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"feishu-mem/internal/decision"
	"feishu-mem/internal/storage/bitable"
	gitstorage "feishu-mem/internal/storage/git"
	larkadapter "feishu-mem/internal/lark-adapter"
)

// SyncManager Git ↔ Bitable 双向同步管理器
type SyncManager struct {
	gitStore      *gitstorage.GitStorage
	bitableStore  *bitable.BitableStore
	gitCLI         *gitstorage.GitCLI
	larkCLI         *larkadapter.LarkCLI
	baseToken      string
	tableID          string
	dataDir          string
	changeLog        []ChangeLogEntry
	mu sync.Mutex
}

// ChangeLogEntry 变更日志条目
type ChangeLogEntry struct {
	Time     time.Time `json:"time"`
	Source   string    `json:"source"`   // "git" or "bitable"
	Op       string    `json:"op"`       // "create", "update", "delete"
	SDRID    string    `json:"sdr_id"`
	Title    string    `json:"title"`
	Details  string    `json:"details"`
}

// NewSyncManager 创建同步管理器
func NewSyncManager(gitStore *gitstorage.GitStorage, bitableStore *bitable.BitableStore, gitCLI *gitstorage.GitCLI, larkCLI *larkadapter.LarkCLI, baseToken, tableID, dataDir string) *SyncManager {
	return &SyncManager{
		gitStore:     gitStore,
		bitableStore: bitableStore,
		gitCLI:       gitCLI,
		larkCLI:       larkCLI,
		baseToken:    baseToken,
		tableID:        tableID,
		dataDir:        dataDir,
		changeLog:  []ChangeLogEntry{},
	}
}

// LogChange 记录变更
func (sm *SyncManager) LogChange(source, op, sdrID, title, details string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	entry := ChangeLogEntry{
		Time:    time.Now(),
		Source:  source,
		Op:      op,
		SDRID:   sdrID,
		Title:   title,
		Details: details,
	}
	sm.changeLog = append(sm.changeLog, entry)
	log.Printf("[Sync] %s %s: %s - %s", source, op, sdrID, title)
}

// GetChangeLog 获取变更日志
func (sm *SyncManager) GetChangeLog() []ChangeLogEntry {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return append([]ChangeLogEntry{}, sm.changeLog...)
}

// ========== 正向同步：Git → Bitable ==========

// SyncGitToBitable 同步 Git 变更到 Bitable
func (sm *SyncManager) SyncGitToBitable() error {
	log.Println("[Sync] === 开始正向同步: Git → Bitable ===")

	decisions, err := sm.readAllGitDecisions()
	if err != nil {
		return fmt.Errorf("读取 Git 决策失败: %w", err)
	}
	log.Printf("[Sync] Git 中有 %d 个决策", len(decisions))

	bitableDecisions, err := sm.bitableStore.ListAllDecisions()
	if err != nil {
		log.Printf("[Sync] 读取 Bitable 决策失败: %v", err)
		bitableDecisions = []*decision.DecisionNode{}
	}
	log.Printf("[Sync] Bitable 中有 %d 个决策", len(bitableDecisions))

	bitableIndex := make(map[string]*decision.DecisionNode)
	for _, d := range bitableDecisions {
		if d.SDRID != "" {
			bitableIndex[d.SDRID] = d
			log.Printf("[Sync] Bitable 中已有: %s (hash: %q)", d.SDRID, d.GitCommitHash)
		}
	}

	for _, d := range decisions {
		log.Printf("[Sync] 处理 Git 决策: %s (%s)", d.SDRID, d.Title)
		log.Printf("[Sync]   Git 决策 hash: %q", d.GitCommitHash)

		if existing, ok := bitableIndex[d.SDRID]; ok {
			log.Printf("[Sync]   Bitable hash: %q", existing.GitCommitHash)
			if existing.GitCommitHash != d.GitCommitHash {
				log.Printf("[Sync]   Hash 不一致，更新 Bitable")
				err = sm.bitableStore.UpsertDecision(d)
				if err != nil {
					log.Printf("[Sync]   更新失败: %v", err)
				} else {
					sm.LogChange("git", "update", d.SDRID, d.Title, "Git hash: "+d.GitCommitHash)
				}
			} else {
				log.Printf("[Sync]   Hash 一致，跳过")
			}
		} else {
			log.Printf("[Sync]   Bitable 中不存在，插入新记录")
			err = sm.bitableStore.UpsertDecision(d)
			if err != nil {
				log.Printf("[Sync]   插入失败: %v", err)
			} else {
				sm.LogChange("git", "create", d.SDRID, d.Title, "Git hash: "+d.GitCommitHash)
			}
		}
	}

	log.Println("[Sync] === 正向同步完成 ===")
	return nil
}

// ========== 反向同步：Bitable → Git ==========

// SyncBitableToGit 同步 Bitable 变更到 Git
func (sm *SyncManager) SyncBitableToGit() error {
	log.Println("[Sync] === 开始反向同步: Bitable → Git ===")

	bitableDecisions, err := sm.bitableStore.ListAllDecisions()
	if err != nil {
		return fmt.Errorf("读取 Bitable 决策失败: %w", err)
	}
	log.Printf("[Sync] Bitable 中有 %d 个决策", len(bitableDecisions))

	gitDecisions, err := sm.readAllGitDecisions()
	if err != nil {
		log.Printf("[Sync] 读取 Git 决策失败: %v", err)
		gitDecisions = []*decision.DecisionNode{}
	}
	log.Printf("[Sync] Git 中有 %d 个决策", len(gitDecisions))

	gitIndex := make(map[string]*decision.DecisionNode)
	for _, d := range gitDecisions {
		if d.SDRID != "" {
			gitIndex[d.SDRID] = d
		}
	}

	for _, d := range bitableDecisions {
		if d.SDRID == "" {
			log.Printf("[Sync] 跳过无 sdr_id 的记录")
			continue
		}
		log.Printf("[Sync] 处理 Bitable 决策: %s (%s)", d.SDRID, d.Title)
		log.Printf("[Sync]   Bitable 决策 hash: %q", d.GitCommitHash)

		if d.Project == "" {
			d.Project = "feishu-mem"
		}
		if d.Topic == "" {
			d.Topic = "general"
		}

		if existing, ok := gitIndex[d.SDRID]; ok {
			gitHash, _ := sm.getGitFileHash(d.Project, d.Topic, d.SDRID)
			log.Printf("[Sync]   Git 中已存在, Git hash: %s", gitHash)

			if sm.hasChanges(existing, d) {
				log.Printf("[Sync]   内容有变化，更新 Git")
				updated := sm.mergeDecisions(existing, d)
				commitHash, err := sm.gitStore.WriteDecision(updated)
				if err != nil {
					log.Printf("[Sync]   更新失败: %v", err)
				} else {
					sm.LogChange("bitable", "update", d.SDRID, d.Title, "Commit: "+commitHash)
					log.Printf("[Sync]   同步更新 Bitable 中的 hash")
					updated.GitCommitHash = commitHash
					sm.bitableStore.UpsertDecision(updated)
				}
			} else {
				log.Printf("[Sync]   内容无变化，跳过")
				if d.GitCommitHash == "" && gitHash != "" {
					log.Printf("[Sync]   补全 Bitable 中的 git_commit_hash")
					existing.GitCommitHash = gitHash
					sm.bitableStore.UpsertDecision(existing)
				}
			}
		} else {
			log.Printf("[Sync]   Git 中不存在，写入新文件")
			commitHash, err := sm.gitStore.WriteDecision(d)
			if err != nil {
				log.Printf("[Sync]   写入失败: %v", err)
			} else {
				sm.LogChange("bitable", "create", d.SDRID, d.Title, "Commit: "+commitHash)
				log.Printf("[Sync]   同步更新 Bitable 中的 hash")
				d.GitCommitHash = commitHash
				sm.bitableStore.UpsertDecision(d)
			}
		}
	}

	log.Println("[Sync] === 反向同步完成 ===")
	return nil
}

// ========== 辅助方法 ==========

// readAllGitDecisions 读取 Git 中所有决策
func (sm *SyncManager) readAllGitDecisions() ([]*decision.DecisionNode, error) {
	var allDecisions []*decision.DecisionNode

	decisionsDir := filepath.Join(sm.dataDir, "decisions")
	if _, err := os.Stat(decisionsDir); os.IsNotExist(err) {
		return allDecisions, nil
	}

	projectEntries, err := os.ReadDir(decisionsDir)
	if err != nil {
		return nil, err
	}

	for _, projEntry := range projectEntries {
		if !projEntry.IsDir() {
			continue
		}
		project := projEntry.Name()

		topicDir := filepath.Join(decisionsDir, project)
		topicEntries, err := os.ReadDir(topicDir)
		if err != nil {
			continue
		}

		for _, topicEntry := range topicEntries {
			if !topicEntry.IsDir() {
				continue
			}
			topic := topicEntry.Name()

			decisions, err := sm.gitStore.ListDecisions(project, topic)
			if err != nil {
				log.Printf("[Sync]   读取失败: %v", err)
				continue
			}

			for _, d := range decisions {
				if d.GitCommitHash == "" {
					hash, err := sm.getGitFileHash(project, topic, d.SDRID)
					if err == nil {
						d.GitCommitHash = hash
					}
				}
			}

			allDecisions = append(allDecisions, decisions...)
		}
	}

	return allDecisions, nil
}

// getGitFileHash 获取文件在 Git 中的 hash
func (sm *SyncManager) getGitFileHash(project, topic, sdrID string) (string, error) {
	relPath := fmt.Sprintf("decisions/%s/%s/%s.md", project, topic, sdrID)
	hash, err := sm.gitCLI.Run("log", "-1", "--format=%H", "--", relPath)
	if err != nil {
		if strings.Contains(err.Error(), "exit status 128") {
			return sm.gitCLI.GetHeadHash()
		}
		return "", err
	}
	if hash == "" {
		return sm.gitCLI.GetHeadHash()
	}
	return hash, nil
}

// hasChanges 比较两个决策是否有变化
func (sm *SyncManager) hasChanges(a, b *decision.DecisionNode) bool {
	if a.Title != b.Title {
		return true
	}
	if a.Decision != b.Decision {
		return true
	}
	if a.Rationale != b.Rationale {
		return true
	}
	if a.Status != b.Status {
		return true
	}
	if a.ImpactLevel != b.ImpactLevel {
		return true
	}
	if a.Topic != b.Topic {
		return true
	}
	if a.Proposer != b.Proposer {
		return true
	}
	if a.Executor != b.Executor {
		return true
	}
	return false
}

// mergeDecisions 合并决策
func (sm *SyncManager) mergeDecisions(gitNode, bitableNode *decision.DecisionNode) *decision.DecisionNode {
	merged := *gitNode

	if bitableNode.Title != "" {
		merged.Title = bitableNode.Title
	}
	if bitableNode.Decision != "" {
		merged.Decision = bitableNode.Decision
	}
	if bitableNode.Rationale != "" {
		merged.Rationale = bitableNode.Rationale
	}
	if bitableNode.Status != "" {
		merged.Status = bitableNode.Status
	}
	if bitableNode.ImpactLevel != "" {
		merged.ImpactLevel = bitableNode.ImpactLevel
	}
	if bitableNode.Topic != "" {
		merged.Topic = bitableNode.Topic
	}
	if bitableNode.Proposer != "" {
		merged.Proposer = bitableNode.Proposer
	}
	if bitableNode.Executor != "" {
		merged.Executor = bitableNode.Executor
	}
	if len(bitableNode.CrossTopicRefs) > 0 {
		merged.CrossTopicRefs = bitableNode.CrossTopicRefs
	}
	if len(bitableNode.Stakeholders) > 0 {
		merged.Stakeholders = bitableNode.Stakeholders
	}
	if len(bitableNode.FeishuLinks.RelatedChatIDs) > 0 {
		merged.FeishuLinks.RelatedChatIDs = bitableNode.FeishuLinks.RelatedChatIDs
	}
	return &merged
}

// ========== 直接操作方法 ==========

// CreateInGit 在 Git 中创建决策
func (sm *SyncManager) CreateInGit(node *decision.DecisionNode) (string, error) {
	log.Printf("[Sync] CreateInGit: %s", node.SDRID)
	hash, err := sm.gitStore.WriteDecision(node)
	if err != nil {
		return "", err
	}
	node.GitCommitHash = hash
	sm.LogChange("git", "create", node.SDRID, node.Title, "Commit: "+hash)
	return hash, nil
}

// UpdateInGit 在 Git 中更新决策
func (sm *SyncManager) UpdateInGit(node *decision.DecisionNode) (string, error) {
	log.Printf("[Sync] UpdateInGit: %s", node.SDRID)
	hash, err := sm.gitStore.WriteDecision(node)
	if err != nil {
		return "", err
	}
	node.GitCommitHash = hash
	sm.LogChange("git", "update", node.SDRID, node.Title, "Commit: "+hash)
	return hash, nil
}

// DeleteInGit 在 Git 中删除决策
func (sm *SyncManager) DeleteInGit(project, topic, sdrID string) error {
	log.Printf("[Sync] DeleteInGit: %s", sdrID)
	path := fmt.Sprintf("decisions/%s/%s/%s.md", project, topic, sdrID)
	fullPath := filepath.Join(sm.dataDir, path)
	if _, err := os.Stat(fullPath); err == nil {
		if _, err := sm.gitCLI.Run("rm", path); err != nil {
			os.Remove(fullPath)
		}
		_, err := sm.gitCLI.Commit(".", fmt.Sprintf("delete: %s", sdrID))
		if err != nil && !strings.Contains(err.Error(), "nothing to commit") {
			return err
		}
		sm.LogChange("git", "delete", sdrID, "", "Deleted from Git")
	}
	return nil
}

// CreateInBitable 在 Bitable 中创建决策
func (sm *SyncManager) CreateInBitable(node *decision.DecisionNode) error {
	log.Printf("[Sync] CreateInBitable: %s", node.SDRID)
	err := sm.bitableStore.UpsertDecision(node)
	if err != nil {
		return err
	}
	sm.LogChange("bitable", "create", node.SDRID, node.Title, "Added to Bitable")
	return nil
}

// UpdateInBitable 在 Bitable 中更新决策
func (sm *SyncManager) UpdateInBitable(node *decision.DecisionNode) error {
	log.Printf("[Sync] UpdateInBitable: %s", node.SDRID)
	err := sm.bitableStore.UpsertDecision(node)
	if err != nil {
		return err
	}
	sm.LogChange("bitable", "update", node.SDRID, node.Title, "Updated in Bitable")
	return nil
}

// DeleteInBitable 在 Bitable 中删除决策
func (sm *SyncManager) DeleteInBitable(sdrID string) error {
	log.Printf("[Sync] DeleteInBitable: %s", sdrID)
	decisions, err := sm.bitableStore.ListAllDecisions()
	if err != nil {
		return err
	}
	for _, d := range decisions {
		if d.SDRID == sdrID {
			output, err := sm.larkCLI.RunCommand("base", "+record-list", "--base-token", sm.baseToken, "--table-id", sm.tableID)
			if err != nil {
				return err
			}
			var result map[string]any
			json.Unmarshal(output, &result)
			if items, ok := result["items"].([]any); ok {
				for _, item := range items {
					if rec, ok := item.(map[string]any); ok {
						if fields, ok := rec["fields"].(map[string]any); ok {
							if id, ok := fields["sdr_id"].(string); ok && id == sdrID {
								if recordID, ok := rec["record_id"].(string); ok {
									_, err := sm.larkCLI.RunCommand("base", "+record-delete", "--base-token", sm.baseToken, "--table-id", sm.tableID, "--record-id", recordID, "--yes")
									if err != nil {
										return err
									}
									sm.LogChange("bitable", "delete", sdrID, "", "Deleted from Bitable")
									return nil
								}
							}
						}
					}
				}
			}
		}
	}
	return fmt.Errorf("record not found: %s", sdrID)
}

// ListFromGit 列出 Git 中的所有决策
func (sm *SyncManager) ListFromGit() ([]*decision.DecisionNode, error) {
	return sm.readAllGitDecisions()
}

// ListFromBitable 列出 Bitable 中的所有决策
func (sm *SyncManager) ListFromBitable() ([]*decision.DecisionNode, error) {
	return sm.bitableStore.ListAllDecisions()
}
