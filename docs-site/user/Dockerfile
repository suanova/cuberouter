# CubeRouter 用户文档 - 使用本地已构建的静态文件
# 用法: 先在 docs-site/user 目录执行 bun install && bun run build
# 然后: docker build -t cuberouter-docs-user:latest .
FROM nginx:alpine
COPY docs/.vitepress/dist /usr/share/nginx/html
# 确保 nginx worker 用户可读（部分截图文件权限为 600）
RUN chmod -R a+rX /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
