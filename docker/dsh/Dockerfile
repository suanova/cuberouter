# CubeRouter DSH 运行时镜像
#
# DeepSeek Harness（dsh）以 Web 模式运行，供 CubeRouter 的 DSH 任务渠道调用，
# 同时自带 Office（PPT/DOCX/XLSX）技能，可直接产出演示文稿。
#
# 构建：docker compose build dsh
# 运行：见仓库根目录 docker-compose.yml 中 dsh 服务；数据卷挂到 /data。

FROM node:24-bookworm-slim

ARG DSH_VERSION=0.2.2-rc.1
ARG DSH_SKILL_OFFICE_VERSION=0.2.2-rc.1

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates python3 python3-pip wget \
    && rm -rf /var/lib/apt/lists/* \
    && pip3 install --no-cache-dir --break-system-packages \
        python-pptx python-docx openpyxl pillow

RUN npm install -g "@deepseek-ai/dsh@${DSH_VERSION}" pnpm@11

# 插件：dsh-api（CubeRouter 的对外任务 API，含产物下载）与 Office 技能。
# 种子 profile 固定装在 /opt/dsh-seed，首次启动由 entrypoint 复制到数据卷。
#
# pnpm 11 遇到未放行的依赖构建脚本会直接失败（Office 技能经 libreoffice-kit
# 间接依赖 koffi）。koffi 的平台预编译包 @koromix/koffi-<platform> 已随包提供
# koffi.node，无需执行其 install 脚本，因此在 profile 里先写好包含 allowBuilds
# 的 pnpm 设置；文件必须在首次 `dsh plugin add` 之前存在，initProfile 只补齐
# 缺失文件、不会覆盖它，同时保证 node_modules 采用新装即用的 hoisted 布局。
COPY plugins/dsh-api /opt/dsh-plugins/dsh-api
COPY pnpm-workspace.yaml /opt/dsh-seed/profiles/web/pnpm-workspace.yaml
RUN DSH_HOME=/opt/dsh-seed dsh plugin --profile web add /opt/dsh-plugins/dsh-api \
    && DSH_HOME=/opt/dsh-seed dsh plugin --profile web add \
        "@deepseek-ai/dsh-skill-office@${DSH_SKILL_OFFICE_VERSION}"

COPY overlay.yml /opt/dsh/overlay.yml
COPY workspace-AGENTS.md /opt/dsh/workspace-AGENTS.md
COPY entrypoint.sh /usr/local/bin/dsh-entrypoint.sh
RUN chmod +x /usr/local/bin/dsh-entrypoint.sh

EXPOSE 3080
ENTRYPOINT ["/usr/local/bin/dsh-entrypoint.sh"]
