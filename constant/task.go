package constant

type TaskPlatform string

const (
	TaskPlatformSuno       TaskPlatform = "suno"
	TaskPlatformMidjourney              = "mj"
)

const (
	// TaskActionGenerate 是视频生成请求的默认动作（legacy 别名 "generate"，
	// NormalizeTaskAction 会归一到 image_to_video）。fork 保留的 Go 任务适配器
	// （doubao/astraflow）使用该动作，上游因迁移到 JS 插件系统而移除了它。
	TaskActionGenerate          = "generate"
	// TaskActionTextGenerate 是 Kling/Jimeng 旧路由中间件写入的 legacy 动作
	// 别名（NormalizeTaskAction 会归一到 text_to_video）。
	TaskActionTextGenerate      = "textGenerate"
	TaskActionImageToVideo     = "image_to_video"
	TaskActionTextToVideo      = "text_to_video"
	TaskActionFirstTailToVideo = "first_tail_to_video"
	TaskActionReferenceToVideo = "reference_to_video"
	TaskActionRemix            = "remix"
)

var legacyTaskActionAliases = map[string]string{
	"generate":          TaskActionImageToVideo,
	"textGenerate":      TaskActionTextToVideo,
	"firstTailGenerate": TaskActionFirstTailToVideo,
	"referenceGenerate": TaskActionReferenceToVideo,
	"remixGenerate":     TaskActionRemix,
}

// TaskPluginEnabled is the master switch for the whole task-plugin system.
// When disabled, factory and override plugins both stop serving.
var TaskPluginEnabled = true

// TaskPluginOverrideEnabled controls whether the database override layer is
// active. When disabled, uploaded plugins are ignored and factory plugins are
// used instead; the factory layer is unaffected.
var TaskPluginOverrideEnabled = true

// NormalizeTaskAction maps persisted legacy action names to the canonical task
// action vocabulary. Unknown platform-specific actions pass through unchanged.
func NormalizeTaskAction(action string) string {
	if canonical, ok := legacyTaskActionAliases[action]; ok {
		return canonical
	}
	return action
}
