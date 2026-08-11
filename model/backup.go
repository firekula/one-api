package model

import (
	"github.com/songquanpeng/one-api/common/helper"
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
