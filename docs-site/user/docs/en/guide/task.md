# Task Logs

> Manage the status and results of async tasks such as Midjourney image drawing and Suno music generation. Click **Task Logs** in the left navigation, or go directly to `/usage-logs/task`.

![Task Logs](/imgs-en/task.jpeg)

The Task Logs page provides two tabs:

- **Drawing Logs**: view records of drawing tasks (Midjourney, etc.)
- **Task Logs**: view records of other async tasks such as music generation (Suno, etc.)

## View the Task List

The task list shows all submitted async generation tasks with the following information:

- **Submitted at**: when the task was submitted
- **Task ID**: the unique identifier of the task
- **Duration**: how long the task took to execute
- **Status**: the current status of the task
- **Progress**: the task's completion progress
- **Details**: view the detailed results of the task

You can filter for a specific task using the **Task ID** filter condition.

## Task Status Explained

| Status | Description |
| --- | --- |
| `PENDING` | Task submitted, waiting to be processed |
| `IN_PROGRESS` | Task is generating |
| `SUCCESS` | Task completed, results available |
| `FAILURE` | Task generation failed; quota has been refunded automatically |
