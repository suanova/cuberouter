package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// ImportChannelModelsCsv 上传 UCloud CSV,为指定渠道整合导入:模型 + 价格 + 介绍。
// @Summary  CSV 批量导入渠道模型/价格/介绍
// @Tags     渠道-模型导入
// @Security ApiKeyAuth
// @Accept   multipart/form-data
// @Produce  json
// @Param    id   path   int    true "渠道 ID"
// @Param    file formData file   true "UCloud CSV 文件(UTF-8)"
// @Success  200 {object} dto.ImportCsvAPIResponse "data 为 dto.ImportCsvResult 结构化结果"
// @Failure  400 {object} dto.APIResponse "invalid channel id / no_file / bad_header / invalid_csv_row / too_many_rows / bad_encoding / channel_not_found"
// @Failure  413 {object} dto.APIResponse "file_too_large"
// @Failure  500 {object} dto.APIResponse
// @Router   /channel/{id}/import_models_csv [post]
func ImportChannelModelsCsv(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid channel id"})
		return
	}

	// per-handler 10MB 限制(不动全局 MaxMultipartMemory,避免压缩全站)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": "file_too_large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "no_file"})
		return
	}
	defer file.Close()

	// 表头/FieldsPerRecord/行数/UTF-8 校验在 service 内完成;此处按 sentinel error 映射 HTTP 状态码。
	result, err := service.ImportChannelModelsCSV(id, file)
	if err != nil {
		if errors.Is(err, service.ErrChannelNotFound) ||
			errors.Is(err, service.ErrBadHeader) ||
			errors.Is(err, service.ErrInvalidCsvRow) ||
			errors.Is(err, service.ErrTooManyRows) ||
			errors.Is(err, service.ErrBadEncoding) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}
