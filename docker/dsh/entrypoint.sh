#!/bin/sh
# DSH 容器入口：首次启动把镜像内置的 profile 种子复制到数据卷，准备 Agent
# 工作目录，然后以容器网卡地址:3080 启动 Web 服务（端口/网卡由 overlay.yml 固定）。
set -eu

export DSH_HOME="${DSH_HOME:-/data}"

if [ ! -f "$DSH_HOME/profiles/web/package.json" ]; then
    mkdir -p "$DSH_HOME/profiles"
    cp -a /opt/dsh-seed/profiles/. "$DSH_HOME/profiles/"
fi

# pnpm 以相对路径记录本地插件链接；数据卷深度变化后可能失效，统一改回镜像内绝对路径。
if [ -L "$DSH_HOME/profiles/web/node_modules/dsh-api" ]; then
    ln -sfn /opt/dsh-plugins/dsh-api "$DSH_HOME/profiles/web/node_modules/dsh-api"
fi

export DSH_WORKSPACE="${DSH_WORKSPACE:-$DSH_HOME/workspace}"
mkdir -p "$DSH_WORKSPACE"
if [ ! -f "$DSH_WORKSPACE/AGENTS.md" ]; then
    cp /opt/dsh/workspace-AGENTS.md "$DSH_WORKSPACE/AGENTS.md"
fi

# 按用户拆分的任务目录（$DSH_WORKSPACE/<userId>）不是共享根的子孙项目，
# AGENTS.md 的项目根发现走不到共享根那份约定；DSH_HOME/AGENTS.md 是 user-global
# 指令文件，任何 cwd 都会加载。共享根那份保留给旧任务。
if [ ! -f "$DSH_HOME/AGENTS.md" ]; then
    cp /opt/dsh/workspace-AGENTS.md "$DSH_HOME/AGENTS.md"
fi

# dsh 0.2.2 起拒绝 0.0.0.0 等通配地址，只接受具体网卡的 IP 字面量。容器内取
# 第一个非回环 IPv4（即 eth0），并通过 DSH_BIND_HOST 交给 overlay.yml；同时把
# 它写到文件里，供 compose healthcheck 探活。
if [ -z "${DSH_BIND_HOST:-}" ]; then
    DSH_BIND_HOST=$(node -e 'const n = require("node:os").networkInterfaces(); for (const list of Object.values(n)) for (const a of list || []) if (a.family === "IPv4" && !a.internal) { process.stdout.write(a.address); process.exit(0) } process.exit(1)') || DSH_BIND_HOST=127.0.0.1
    export DSH_BIND_HOST
fi
printf '%s\n' "$DSH_BIND_HOST" > /tmp/dsh-bind-host

# 启动器自己的选项（--patch）必须排在应用参数之前：解析器遇到第一个不认识的
# 参数后，其余全部原样交给 web 应用（--no-open 等）。
set -- web --patch /opt/dsh/overlay.yml --no-open

# 通过非 loopback 主机名访问 Web UI 时，浏览器信任栅栏要求显式声明该 authority
# （空格分隔的 host 或 host:port 列表）。
# shellcheck disable=SC2086
if [ -n "${DSH_TRUSTED_HOST:-}" ]; then
    set -- "$@" --trusted-host $DSH_TRUSTED_HOST
fi

exec dsh "$@"
