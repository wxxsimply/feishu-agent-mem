package larkadapter

import (
	"encoding/json"
	"fmt"
	"time"
)

// VCExtractor 视频会议提取器
type VCExtractor struct {
	config *Config
	cli    *LarkCLI
}

// NewVCExtractor 创建视频会议提取器
func NewVCExtractor(cfg *Config) *VCExtractor {
	return &VCExtractor{
		config: cfg,
		cli:    NewLarkCLI(),
	}
}

// Name 实现 Extractor 接口
func (e *VCExtractor) Name() string {
	return "lark_vc"
}

// Detect 检测会议和妙记变化（新会议、会议结束、会议纪要、妙记更新等）
func (e *VCExtractor) Detect(lastCheck time.Time) (*DetectResult, error) {
	changes := []Change{}
	cutoff := lastCheck.Unix()

	if !lastCheck.IsZero() {
		// 检测会议变化
		vcOutput, err := e.cli.RunCommand("vc", "+search")
		if err == nil {
			meetings := e.parseMeetingsWithDetails(vcOutput, cutoff)
			changes = append(changes, meetings...)
		}

		// 检测妙记变化
		minutesOutput, err := e.cli.RunCommand("minutes", "+search", "--query", "")
		if err == nil {
			minutes := e.parseMinutesWithDetails(minutesOutput, cutoff)
			changes = append(changes, minutes...)
		}
	}

	result := &DetectResult{
		Source:     e.Name(),
		HasChanges: len(changes) > 0,
		DetectedAt: time.Now(),
		LastCheck:  lastCheck,
		Changes:    changes,
	}

	_ = SaveDetectResult(result)
	return result, nil
}

// parseMeetingsWithDetails 解析会议并识别各种变化类型
func (e *VCExtractor) parseMeetingsWithDetails(output []byte, cutoff int64) []Change {
	var changes []Change

	var result []any
	if err := json.Unmarshal(output, &result); err != nil {
		var single any
		if err := json.Unmarshal(output, &single); err != nil {
			return changes
		}
		result = []any{single}
	}

	for _, item := range result {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		data, ok := itemMap["data"].(map[string]any)
		if !ok {
			continue
		}
		items, ok := data["items"].([]any)
		if !ok {
			continue
		}
		for _, m := range items {
			mMap, ok := m.(map[string]any)
			if !ok {
				continue
			}
			meetingID, _ := mMap["meeting_id"].(string)
			topic, _ := mMap["topic"].(string)
			if meetingID == "" {
				continue
			}

			// 获取会议时间
			var meetingTs int64
			if endTime, ok := mMap["end_time"].(string); ok {
				meetingTs = parseMessageTime(endTime)
			} else if createTime, ok := mMap["create_time"].(string); ok {
				meetingTs = parseMessageTime(createTime)
			}

			// 只处理时间范围内的会议
			if meetingTs <= cutoff {
				continue
			}

			// 分析会议状态
			status, _ := mMap["status"].(string)

			switch status {
			case "ended":
				changes = append(changes, Change{
					Type:       "meeting_ended",
					EntityType: "meeting",
					EntityID:   meetingID,
					Summary:    fmt.Sprintf("会议已结束: %s", topic),
					Timestamp:  meetingTs,
				})

				// 检查是否有会议纪要
				if noteDoc, ok := mMap["note_doc"].(map[string]any); ok && noteDoc != nil {
					changes = append(changes, Change{
						Type:       "meeting_minutes_available",
						EntityType: "meeting_minutes",
						EntityID:   meetingID,
						Summary:    fmt.Sprintf("会议纪要生成: %s", topic),
						Timestamp:  meetingTs,
					})
				}

			case "upcoming":
				changes = append(changes, Change{
					Type:       "meeting_scheduled",
					EntityType: "meeting",
					EntityID:   meetingID,
					Summary:    fmt.Sprintf("新会议安排: %s", topic),
					Timestamp:  meetingTs,
				})

			case "recording":
				changes = append(changes, Change{
					Type:       "meeting_recording",
					EntityType: "meeting",
					EntityID:   meetingID,
					Summary:    fmt.Sprintf("会议录制中: %s", topic),
					Timestamp:  meetingTs,
				})

			default:
				changes = append(changes, Change{
					Type:       "meeting_updated",
					EntityType: "meeting",
					EntityID:   meetingID,
					Summary:    fmt.Sprintf("会议更新: %s", topic),
					Timestamp:  meetingTs,
				})
			}

			// 检查是否有录制文件
			if recording, ok := mMap["recording"].(map[string]any); ok && recording != nil {
				changes = append(changes, Change{
					Type:       "meeting_recording_available",
					EntityType: "meeting_recording",
					EntityID:   meetingID,
					Summary:    fmt.Sprintf("会议录制文件就绪: %s", topic),
					Timestamp:  meetingTs,
				})
			}
		}
	}

	return changes
}

// parseMinutesWithDetails 解析妙记并识别各种变化类型
func (e *VCExtractor) parseMinutesWithDetails(output []byte, cutoff int64) []Change {
	var changes []Change

	var result []any
	if err := json.Unmarshal(output, &result); err != nil {
		var single any
		if err := json.Unmarshal(output, &single); err != nil {
			return changes
		}
		result = []any{single}
	}

	for _, item := range result {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		data, ok := itemMap["data"].(map[string]any)
		if !ok {
			continue
		}
		items, ok := data["items"].([]any)
		if !ok {
			continue
		}
		for _, m := range items {
			mMap, ok := m.(map[string]any)
			if !ok {
				continue
			}
			token, _ := mMap["minute_token"].(string)
			title, _ := mMap["title"].(string)
			if token == "" {
				continue
			}

			// 获取时间戳
			var minuteTs int64
			if createTime, ok := mMap["create_time"].(string); ok {
				minuteTs = parseMessageTime(createTime)
			} else if updateTime, ok := mMap["update_time"].(string); ok {
				minuteTs = parseMessageTime(updateTime)
			}

			if minuteTs <= cutoff {
				continue
			}

			// 检查是否是新创建的
			if createTime, ok := mMap["create_time"].(string); ok {
				createTs := parseMessageTime(createTime)
				if createTs > cutoff {
					changes = append(changes, Change{
						Type:       "minutes_created",
						EntityType: "minutes",
						EntityID:   token,
						Summary:    fmt.Sprintf("新妙记生成: %s", title),
						Timestamp:  createTs,
					})
				}
			}

			// 检查内容更新
			if updateTime, ok := mMap["update_time"].(string); ok {
				updateTs := parseMessageTime(updateTime)
				if createTime, ok := mMap["create_time"].(string); ok {
					createTs := parseMessageTime(createTime)
					if updateTs > cutoff && updateTs > createTs {
						changes = append(changes, Change{
							Type:       "minutes_updated",
							EntityType: "minutes",
							EntityID:   token,
							Summary:    fmt.Sprintf("妙记内容更新: %s", title),
							Timestamp:  updateTs,
						})
					}
				}
			}

			// 检查 AI 摘要状态
			if aiSummary, ok := mMap["ai_summary"].(map[string]any); ok && aiSummary != nil {
				changes = append(changes, Change{
					Type:       "minutes_ai_summary_ready",
					EntityType: "minutes",
					EntityID:   token,
					Summary:    fmt.Sprintf("妙记 AI 摘要就绪: %s", title),
					Timestamp:  minuteTs,
				})
			}

			// 检查是否有录制内容
			if hasRecording, ok := mMap["has_recording"].(bool); ok && hasRecording {
				changes = append(changes, Change{
					Type:       "minutes_recording_available",
					EntityType: "minutes",
					EntityID:   token,
					Summary:    fmt.Sprintf("妙记有录制内容: %s", title),
					Timestamp:  minuteTs,
				})
			}
		}
	}

	return changes
}

// Extract 提取会议决策信息
func (e *VCExtractor) Extract() error {
	rawData := make(map[string]any)
	errors := make(map[string]string)

	if meetings, err := e.searchMeetings(); err == nil {
		rawData["meetings"] = meetings
	} else {
		errors["meetings"] = err.Error()
	}

	if len(errors) > 0 {
		rawData["_errors"] = errors
	}

	formatted := map[string]any{
		"extracted": true,
	}

	result := &ExtractionResult{
		Source:      e.Name(),
		ExtractedAt: time.Now(),
		RawData:     rawData,
		Formatted:   formatted,
	}

	if err := SaveToJSON(e.Name(), result); err != nil {
		return fmt.Errorf("save result failed: %w", err)
	}

	return nil
}

func (e *VCExtractor) searchMeetings() ([]any, error) {
	output, err := e.cli.RunCommand("vc", "+search", "--page-all")
	if err != nil {
		return nil, err
	}

	var result []any
	if err := json.Unmarshal(output, &result); err != nil {
		var single any
		if err := json.Unmarshal(output, &single); err != nil {
			return nil, err
		}
		result = []any{single}
	}
	return result, nil
}

func (e *VCExtractor) parseEndedMeetings(output []byte) []map[string]any {
	var meetings []map[string]any

	var result []any
	if err := json.Unmarshal(output, &result); err != nil {
		var single any
		if err := json.Unmarshal(output, &single); err != nil {
			return meetings
		}
		result = []any{single}
	}

	for _, item := range result {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		data, ok := itemMap["data"].(map[string]any)
		if !ok {
			continue
		}
		items, ok := data["items"].([]any)
		if !ok {
			continue
		}
		for _, m := range items {
			mMap, ok := m.(map[string]any)
			if !ok {
				continue
			}
			meetingID, _ := mMap["meeting_id"].(string)
			topic, _ := mMap["topic"].(string)
			if meetingID == "" {
				continue
			}
			meetings = append(meetings, map[string]any{
				"meeting_id": meetingID,
				"topic":      topic,
			})
		}
	}

	return meetings
}
