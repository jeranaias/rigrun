# Background Task System

The Background Task System allows long-running operations to execute without blocking the TUI interface. Users can queue tasks, monitor their progress, receive notifications upon completion, and even switch conversations while tasks run in the background.

## Features

- **Non-blocking execution**: Tasks run in goroutines without freezing the UI
- **Queue management**: Multiple tasks can be queued and run sequentially
- **Real-time notifications**: Get notified when tasks complete, fail, or are canceled
- **Task history**: View completed tasks and their outputs
- **Cancellation**: Stop running tasks at any time
- **Conversation independence**: Switch conversations while tasks run

## Architecture

### Components

1. **Task** (`task.go`): Represents a single background operation
   - ID, description, command, arguments
   - Status tracking (Queued, Running, Complete, Failed, Canceled)
   - Progress tracking (0-100%)
   - Output capturing
   - Thread-safe operations

2. **TaskQueue** (`queue.go`): Manages the task queue
   - Adds tasks to the queue
   - Tracks running tasks
   - Provides filtering (running, completed, etc.)
   - Sends notifications on state changes
   - Cleanup of old tasks

3. **Runner** (`runner.go`): Executes tasks in the background
   - Polls the queue for new tasks
   - Executes tasks in separate goroutines
   - Streams output in real-time
   - Handles task cancellation

4. **TaskList UI** (`ui/components/task_list.go`): Displays tasks in the TUI
   - Lists all tasks with status icons
   - Shows task details (ID, description, duration)
   - Filters tasks by status
   - Detailed task view with output

## Usage

### Commands

#### Create a Task
```
/task "Description" <command> <args...>
```

Examples:
```
/task "Run tests" bash "go test ./..."
/task "Build project" bash "cargo build --release"
/task "Sleep test" sleep "5s"
```

#### List Tasks
```
/tasks [--all|--running|--completed]
```

Examples:
```
/tasks              # Show default view
/tasks --all        # Show all tasks
/tasks --running    # Show only running tasks
/tasks --completed  # Show completed/failed tasks
```

#### Cancel a Task
```
/cancel <task-id>
```

Example:
```
/cancel a1b2c3d4
```

### Supported Commands

#### Bash/Shell Commands
Executes arbitrary shell commands:
```
/task "Run linter" bash "golangci-lint run"
/task "Deploy app" bash "kubectl apply -f deployment.yaml"
```

On Windows, automatically uses PowerShell or cmd if bash is not available.

#### Sleep (for testing)
Simulates a long-running task with progress updates:
```
/task "Wait 10s" sleep "10s"
/task "Wait 2m" sleep "2m"
```

## Integration

### Chat Model Integration

The task system is integrated into the chat model (`ui/chat/model.go`):

```go
// Background task system
taskQueue  *tasks.Queue  // Background task queue
taskRunner *tasks.Runner // Task runner for background execution
```

Initialized in `New()`:
```go
taskQueue := tasks.NewQueue(100) // Keep last 100 completed tasks
taskRunner := tasks.NewRunner(taskQueue)
taskRunner.Start()
```

### Message Handling

Task-related messages are handled in the `Update()` function:

- `TaskCreateMsg`: Creates and queues a new task
- `TaskListMsg`: Shows the task list
- `TaskCancelMsg`: Cancels a running task
- `TaskNotificationMsg`: Displays task completion notifications

### Notifications

Tasks send notifications via a channel when they complete:

```go
// Listen for notifications
select {
case notif := <-m.taskQueue.Notifications():
    // Display notification in chat
default:
    // No notification
}
```

## Thread Safety

All task operations are thread-safe:

- `Task`: Uses `sync.RWMutex` for status, output, and progress
- `Queue`: Uses `sync.RWMutex` for queue operations
- `Runner`: Uses `sync.WaitGroup` for goroutine coordination

## Example Workflow

1. User creates a task:
   ```
   /task "Run tests" bash "go test ./..."
   ```

2. Task is added to the queue with status `Queued`

3. Runner picks up the task and marks it as `Running`

4. Task output is streamed to the task's output buffer

5. Upon completion:
   - Task is marked as `Complete` (or `Failed` if error)
   - Notification is sent to the UI
   - User sees: "✓ Task complete: Run tests [a1b2c3d4] (3.2s)"

6. User can view task details:
   ```
   /tasks
   ```

7. User can switch conversations while task runs in background

## Configuration

Task history limit (in `NewQueue()`):
```go
taskQueue := tasks.NewQueue(100) // Keep last 100 completed tasks
```

Set to `0` for unlimited history (not recommended).

## Testing

Run tests:
```bash
go test ./internal/tasks/...
```

Test coverage includes:
- Task creation and lifecycle
- Status transitions
- Progress tracking
- Queue operations
- Filtering
- Cancellation

## Future Enhancements

- [ ] Task persistence across sessions
- [ ] Task scheduling (run at specific time)
- [ ] Task dependencies (run task B after task A)
- [ ] Resource limits (max concurrent tasks)
- [ ] Task output streaming to UI in real-time
- [ ] Task templates for common operations
- [ ] Task retry with exponential backoff
- [ ] Task priority queue

## Security Considerations

- Tasks inherit the TUI process's permissions
- No sandboxing - tasks can execute arbitrary commands
- User is responsible for validating command safety
- Future: Add permission system for high-risk operations

## Performance

- Minimal overhead: Tasks run in separate goroutines
- Non-blocking: UI remains responsive during task execution
- Efficient: Uses channels for notification, no polling
- Scalable: Supports many concurrent tasks (limited by system resources)
