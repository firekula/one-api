package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/songquanpeng/one-api/model"
)

// GetExportData 导出全部核心业务数据（RootAuth）。返回的 JSON 可直接下载保存。
func GetExportData(c *gin.Context) {
	data, err := model.ExportData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "导出失败：" + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

// ImportData 从上传的 JSON 备份文件导入数据（RootAuth）。
func ImportData(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "未找到上传的备份文件",
		})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无法读取上传的备份文件",
		})
		return
	}
	defer f.Close()
	var data model.BackupData
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无法解析备份文件",
		})
		return
	}
	result, err := model.ImportData(&data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}
