FROM python:3.12-slim
WORKDIR /app
COPY dispatcher.py queue_core.py qwen_channel_bridge.py healthcheck.py ./
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 IMAGE_STUDIO_BIND=0.0.0.0 IMAGE_STUDIO_PORT=18195
USER 10001:10001
EXPOSE 18195
CMD ["python", "-B", "dispatcher.py"]
