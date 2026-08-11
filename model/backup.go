package model

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/utils"
)

const backupSchemaVersion = 1

type BackupTables struct {
	Channels    []*Channel    `json:"channels"`
	Users       []*User       `json:"users"`
	Tokens      []*Token      `json:"tokens"`
	Redemptions []*Redemption `json:"redemptions"`
	Options     []*Option     `json:"options"`
}

type BackupData struct {
	SchemaVersion int          `json:"schemaVersion"`
	ExportedAt    int64        `json:"exportedAt"`
	Data          BackupTables `json:"data"`
}

// ExportData 导出核心业务数据。注意：这是管理员显式备份操作，
// 渠道 Key、用户密码哈希、敏感 option 均按原样导出。
func ExportData() (*BackupData, error) {
	data := &BackupData{
		SchemaVersion: backupSchemaVersion,
		ExportedAt:    helper.GetTimestamp(),
	}
	var err error
	if data.Data.Channels, err = GetAllChannels(0, 0, "all"); err != nil {
		return nil, err
	}
	if data.Data.Options, err = AllOption(); err != nil {
		return nil, err
	}
	if err = DB.Find(&data.Data.Users).Error; err != nil {
		return nil, err
	}
	if err = DB.Find(&data.Data.Tokens).Error; err != nil {
		return nil, err
	}
	if err = DB.Find(&data.Data.Redemptions).Error; err != nil {
		return nil, err
	}
	return data, nil
}

type ImportResult struct {
	Inserted map[string]int `json:"inserted"`
	Skipped  map[string]int `json:"skipped"`
	Failed   []string       `json:"failed"`
}

func keyColumn() string {
	if common.UsingPostgreSQL {
		return `"key"`
	}
	return "`key`"
}

// ImportData 合并导入：按业务唯一键判重，已存在则跳过；id 重建并维护
// 旧 id→新 id 映射还原引用。任一表写入失败则整体回滚。
func ImportData(data *BackupData) (*ImportResult, error) {
	if data == nil {
		return nil, errors.New("备份数据为空")
	}
	if data.SchemaVersion != backupSchemaVersion {
		return nil, errors.New("不支持的备份文件版本")
	}
	result := &ImportResult{
		Inserted: map[string]int{},
		Skipped:  map[string]int{},
		Failed:   []string{},
	}
	keyCol := keyColumn()
	err := DB.Transaction(func(tx *gorm.DB) error {
		channelIDMap := map[int]int{}
		userIDMap := map[int]int{}

		// 1. Channel（含 Ability 重建；AddAbilities 内部用全局 DB，事务内改为直接 tx.Create）
		for _, ch := range data.Data.Channels {
			var count int64
			if err := tx.Model(&Channel{}).Where("type = ? AND "+keyCol+" = ? AND base_url = ?", ch.Type, ch.Key, ch.BaseURL).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				result.Skipped["channels"]++
				continue
			}
			oldID := ch.Id
			ch.Id = 0
			if err := tx.Create(ch).Error; err != nil {
				return err
			}
			channelIDMap[oldID] = ch.Id
			// 重建 Ability（与 AddAbilities 逻辑一致，但使用事务连接）
			models := utils.DeDuplication(strings.Split(ch.Models, ","))
			groups := strings.Split(ch.Group, ",")
			abilities := make([]Ability, 0, len(models)*len(groups))
			for _, model := range models {
				for _, group := range groups {
					abilities = append(abilities, Ability{
						Group:     group,
						Model:     model,
						ChannelId: ch.Id,
						Enabled:   ch.Status == ChannelStatusEnabled,
						Priority:  ch.Priority,
					})
				}
			}
			if len(abilities) > 0 {
				if err := tx.Create(&abilities).Error; err != nil {
					return err
				}
			}
			result.Inserted["channels"]++
		}

		// 2. User（原生 Create 保留 password 哈希/access_token/aff_code 原值）
		for _, u := range data.Data.Users {
			var count int64
			if err := tx.Model(&User{}).Where("username = ?", u.Username).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				result.Skipped["users"]++
				continue
			}
			oldID := u.Id
			u.Id = 0
			if err := tx.Create(u).Error; err != nil {
				return err
			}
			userIDMap[oldID] = u.Id
			result.Inserted["users"]++
		}

		// 3. Token（UserId 经映射转换）
		for _, tk := range data.Data.Tokens {
			var count int64
			if err := tx.Model(&Token{}).Where(keyCol+" = ?", tk.Key).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				result.Skipped["tokens"]++
				continue
			}
			newUserID, ok := userIDMap[tk.UserId]
			if !ok {
				result.Failed = append(result.Failed, fmt.Sprintf("token %q(id=%d): 引用的用户不存在，已跳过", tk.Name, tk.Id))
				continue
			}
			tk.Id = 0
			tk.UserId = newUserID
			if err := tx.Create(tk).Error; err != nil {
				return err
			}
			result.Inserted["tokens"]++
		}

		// 4. Redemption
		for _, r := range data.Data.Redemptions {
			var count int64
			if err := tx.Model(&Redemption{}).Where(keyCol+" = ?", r.Key).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				result.Skipped["redemptions"]++
				continue
			}
			// Redemption.UserId 同样需要经 userIDMap 映射到导入后的新用户 id
			//（r.UserId 为 0 时映射表无此键，保持 0）
			if newUserID, ok := userIDMap[r.UserId]; ok {
				r.UserId = newUserID
			}
			r.Id = 0
			if err := tx.Create(r).Error; err != nil {
				return err
			}
			result.Inserted["redemptions"]++
		}

		// 5. Option（含敏感键；已存在跳过）
		// 注意：这里直接通过事务连接 tx.Create，而不是调用 UpdateOption。
		// UpdateOption 内部使用全局 DB：在 sqlite 下，写锁已被当前事务持有，
		// 事务内经另一连接写库会等待 busy_timeout 后报 database is locked；
		// 且该写入不参与事务回滚。tx.Create 保持导入的原子性。
		for _, o := range data.Data.Options {
			var count int64
			if err := tx.Model(&Option{}).Where(keyCol+" = ?", o.Key).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				result.Skipped["options"]++
				continue
			}
			if err := tx.Create(o).Error; err != nil {
				return err
			}
			result.Inserted["options"]++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 刷新内存渠道缓存与 option 配置（OptionMap/config 变量），让导入立即生效
	InitChannelCache()
	InitOptionMap()
	return result, nil
}
