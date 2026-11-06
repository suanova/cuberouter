# 工作目录约定（DSH Agent 在 /data/workspace 下执行任务时读取）

- 交付物（`.pptx` / `.docx` / `.xlsx` 等）写到当前工作目录，并在最终回复中给出绝对路径。
- Office 文档统一使用系统 `python3`；镜像已预装 `python-pptx`、`python-docx`、
  `openpyxl` 与 `pillow`。`load_workspace_dependencies` 工具在本镜像中不可用，
  请不要安装新的 pip/npm 包，也不要另找解释器。
- 需要渲染预览、导出 PDF 或重算表格时，使用 Office 技能自带的 LibreOffice Kit
  命令行（`dsoffice`，随技能一起安装），不要调用系统 `soffice`，也不要另外
  下载或安装转换引擎。
- 幻灯片中的图片使用工作目录内的本地文件，不要引用网络图片。
