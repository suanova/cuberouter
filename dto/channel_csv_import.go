package dto

// ImportError 表示 CSV 导入过程中某条模型记录的失败原因。
type ImportError struct {
	Model  string `json:"model"`
	Reason string `json:"reason"`
}

// ImportWarning 表示 CSV 导入过程中某条模型记录的非致命提示。
type ImportWarning struct {
	Model  string `json:"model"`
	Reason string `json:"reason"`
}

// ImportCsvResult 汇总一次 CSV 渠道-模型导入的结构化结果。
type ImportCsvResult struct {
	ChannelID              int             `json:"channel_id"`
	ModelsInCSV            int             `json:"models_in_csv"`
	ModelsRecognized       int             `json:"models_recognized"`
	ModelsImported         int             `json:"models_imported"`
	ModelsSkipped          int             `json:"models_skipped"`
	ModelsSkippedNonToken  int             `json:"models_skipped_non_token"`
	ModelsSkippedNoInput   int             `json:"models_skipped_no_input"`
	ModelsFailed           int             `json:"models_failed"`
	IntroUpdated           int             `json:"intro_updated"`
	PriceUpdated           int             `json:"price_updated"`
	PriceSkipped           int             `json:"price_skipped"`
	CompletionRatioSkipped int             `json:"completion_ratio_skipped"`
	RatioPersisted         []string        `json:"ratio_persisted"`
	ChannelUpdateFailed    bool            `json:"channel_update_failed"`
	PersistedRatioModels   []string        `json:"persisted_ratio_models"`
	Errors                 []ImportError   `json:"errors"`
	Warnings               []ImportWarning `json:"warnings"`
}
