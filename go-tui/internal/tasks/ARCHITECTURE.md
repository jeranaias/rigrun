# Background Task System Architecture

## System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         User Interface                       │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Chat TUI                                             │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────────┐  │  │
│  │  │ /task cmd  │  │ /tasks cmd │  │ /cancel cmd    │  │  │
│  │  └─────┬──────┘  └─────┬──────┘  └─────┬──────────┘  │  │
│  │        │               │               │              │  │
│  └────────┼───────────────┼───────────────┼──────────────┘  │
└───────────┼───────────────┼───────────────┼─────────────────┘
            │               │               │
            ▼               ▼               ▼
┌─────────────────────────────────────────────────────────────┐
│                      Chat Model                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Message Handlers                                     │  │
│  │  • handleTaskCreate()                                 │  │
│  │  • handleTaskList()                                   │  │
│  │  • handleTaskCancel()                                 │  │
│  │  • handleTaskNotification()                           │  │
│  └──────────┬───────────────────────────────┬────────────┘  │
└─────────────┼───────────────────────────────┼────────────────┘
              │                               │
              ▼                               │
┌──────────────────────────────────┐          │
│      TaskQueue                   │          │
│  ┌────────────────────────────┐ │          │
│  │  tasks: []*Task            │ │          │
│  │  running: map[string]*Task │ │          │
│  │  notifyChan: chan Notif    │ │◄─────────┘
│  │  mu: sync.RWMutex          │ │   Notifications
│  └────────────────────────────┘ │
│                                  │
│  Methods:                        │
│  • Add(task)                     │
│  • Get(id)                       │
│  • Cancel(id)                    │
│  • Running()                     │
│  • Completed()                   │
│  • Notifications()               │
└──────────┬───────────────────────┘
           │
           │ Polls for queued tasks
           │
           ▼
┌──────────────────────────────────┐
│      Runner                      │
│  ┌────────────────────────────┐ │
│  │  queue: *TaskQueue         │ │
│  │  wg: sync.WaitGroup        │ │
│  │  stop: chan struct{}       │ │
│  └────────────────────────────┘ │
│                                  │
│  Methods:                        │
│  • Start()                       │
│  • Stop()                        │
│  • executeTask()                 │
│  • executeBashTask()             │
│  • executeSleepTask()            │
└──────────┬───────────────────────┘
           │
           │ Spawns goroutines
           │
           ▼
┌──────────────────────────────────┐
│      Task (goroutine)            │
│  ┌────────────────────────────┐ │
│  │  ID: string                │ │
│  │  Description: string       │ │
│  │  Command: string           │ │
│  │  Args: []string            │ │
│  │  Status: TaskStatus        │ │
│  │  StartTime: time.Time      │ │
│  │  EndTime: time.Time        │ │
│  │  Output: string            │ │
│  │  Error: string             │ │
│  │  Progress: int             │ │
│  │  mu: sync.RWMutex          │ │
│  │  cancel: CancelFunc        │ │
│  └────────────────────────────┘ │
│                                  │
│  Methods:                        │
│  • SetStatus()                   │
│  • AppendOutput()                │
│  • SetProgress()                 │
│  • MarkComplete()                │
│  • Cancel()                      │
└──────────────────────────────────┘
```

## Data Flow

### Task Creation Flow

```
User: /task "Description" bash "command"
  │
  ▼
handleTaskCommand()
  │
  ▼
TaskCreateMsg
  │
  ▼
handleTaskCreate()
  │
  ├──► NewTask() ──► Task (status: Queued)
  │
  └──► queue.Add(task)
        │
        └──► listenForNotifications()
```

### Task Execution Flow

```
Runner.processLoop() [ticker: 100ms]
  │
  ├──► queue.Queued()
  │     │
  │     └──► [task1, task2, ...]
  │
  └──► For each task:
        │
        ├──► queue.MarkRunning(task)
        │
        └──► go executeTask(task)
              │
              ├──► Create context with cancel
              │
              ├──► Execute command (bash/sleep)
              │     │
              │     ├──► Stream stdout/stderr
              │     │     │
              │     │     └──► task.AppendOutput()
              │     │
              │     └──► Wait for completion
              │
              └──► Update final status:
                    │
                    ├──► Success: queue.MarkComplete()
                    ├──► Error: queue.MarkFailed()
                    └──► Canceled: queue.MarkCanceled()
                          │
                          └──► Send notification
                                │
                                └──► notifyChan ◄──┐
                                                   │
User sees notification ◄───────────────────────────┘
```

### Task List Flow

```
User: /tasks
  │
  ▼
handleTasksCommand()
  │
  ▼
TaskListMsg{Filter: "default"}
  │
  ▼
handleTaskList()
  │
  ├──► NewTaskList(queue, theme)
  │
  ├──► taskList.SetSize()
  │
  ├──► Configure filter (completed, running, etc.)
  │
  └──► taskList.View()
        │
        ├──► queue.All() ──► Filter by status
        │
        └──► Render:
              │
              ├──► Header
              ├──► Task rows (icon + ID + desc + duration)
              └──► Footer (summary)
```

### Task Cancellation Flow

```
User: /cancel <task-id>
  │
  ▼
handleCancelTaskCommand()
  │
  ▼
TaskCancelMsg{TaskID: "..."}
  │
  ▼
handleTaskCancel()
  │
  └──► queue.Cancel(taskID)
        │
        ├──► Find task in running map
        │
        └──► task.Cancel()
              │
              ├──► Call cancel() ──► Context canceled
              │
              └──► Set status to Canceled
                    │
                    └──► queue.MarkCanceled()
                          │
                          └──► Send notification
```

## Concurrency Model

### Thread Safety

```
Task:
  mu: sync.RWMutex
    ├── Read: GetStatus(), GetOutput(), GetProgress()
    └── Write: SetStatus(), AppendOutput(), SetProgress()

Queue:
  mu: sync.RWMutex
    ├── Read: Get(), Running(), Completed()
    └── Write: Add(), MarkRunning(), MarkComplete()

Runner:
  wg: sync.WaitGroup
    ├── Add(1) when spawning goroutine
    └── Done() when goroutine completes

Notifications:
  notifyChan: buffered channel (100)
    ├── Producer: queue.notify() (on state change)
    └── Consumer: listenForNotifications() (in chat model)
```

### Goroutine Lifecycle

```
Main Goroutine (Chat Model)
  │
  ├──► Runner.Start()
  │     │
  │     └──► processLoop() [long-lived]
  │           │
  │           └──► For each queued task:
  │                 │
  │                 └──► go executeTask() [short-lived]
  │                       │
  │                       ├──► Run command
  │                       └──► Exit when done
  │
  └──► listenForNotifications() [periodic]
        │
        └──► Check notifyChan
              │
              ├──► Has notification: Return TaskNotificationMsg
              └──► No notification: Return nil
```

## State Transitions

```
Task Status State Machine:

       ┌─────────┐
       │ Queued  │◄── Initial state
       └────┬────┘
            │
            ▼
       ┌─────────┐
       │ Running │
       └────┬────┘
            │
            ├──────────┬──────────┬──────────┐
            ▼          ▼          ▼          ▼
       ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐
       │Complete │ │ Failed  │ │Canceled │ │ Queued  │
       └─────────┘ └─────────┘ └─────────┘ └─────────┘
          (final)    (final)     (final)   (can cancel)

Rules:
• Queued → Running (only by Runner)
• Running → Complete/Failed/Canceled (by execution or user)
• Complete/Failed/Canceled → (no transitions, final states)
• Can cancel only if Queued or Running
```

## Components Interaction

```
┌────────────────────────────────────────────────────────┐
│                   Chat Model (TUI)                      │
│  ┌──────────────────────────────────────────────────┐ │
│  │ Responsibilities:                                 │ │
│  │ • Create tasks from user commands                │ │
│  │ • Display task list                              │ │
│  │ • Show notifications                             │ │
│  │ • Handle cancellation requests                   │ │
│  └──────────────────────────────────────────────────┘ │
└───────────┬────────────────────────────┬───────────────┘
            │                            │
      Creates/Queries              Listens for
            │                      notifications
            ▼                            │
┌────────────────────────────────────────┼───────────────┐
│                 TaskQueue              │               │
│  ┌──────────────────────────────────┐ │               │
│  │ Responsibilities:                 │ │               │
│  │ • Store all tasks                 │ │               │
│  │ • Track running tasks             │ │               │
│  │ • Filter tasks by status          │ │               │
│  │ • Send notifications              │◄┘               │
│  │ • Cleanup old tasks               │                 │
│  └──────────────────────────────────┘                 │
└───────────┬────────────────────────────────────────────┘
            │
       Provides tasks
            │
            ▼
┌────────────────────────────────────────────────────────┐
│                     Runner                              │
│  ┌──────────────────────────────────────────────────┐ │
│  │ Responsibilities:                                 │ │
│  │ • Poll queue for new tasks                       │ │
│  │ • Execute tasks in goroutines                    │ │
│  │ • Stream command output                          │ │
│  │ • Handle task completion/failure                 │ │
│  │ • Update queue with results                      │ │
│  └──────────────────────────────────────────────────┘ │
└───────────┬────────────────────────────────────────────┘
            │
       Executes
            │
            ▼
┌────────────────────────────────────────────────────────┐
│                     Task                                │
│  ┌──────────────────────────────────────────────────┐ │
│  │ Responsibilities:                                 │ │
│  │ • Store task metadata                            │ │
│  │ • Track execution status                         │ │
│  │ • Capture output                                 │ │
│  │ • Support cancellation                           │ │
│  │ • Measure duration                               │ │
│  └──────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────┘
```

## Key Design Decisions

1. **Channel-based Notifications**: Non-blocking, efficient, scalable
2. **Thread-safe Operations**: RWMutex for concurrent access
3. **Goroutine per Task**: Parallel execution, simple lifecycle
4. **Context for Cancellation**: Standard Go pattern, works with exec.Command
5. **Buffered Notification Channel**: Prevents blocking on notification send
6. **Separate Runner**: Clear separation of concerns, easy testing
7. **Immutable Task Clones**: Prevent race conditions when reading task state
8. **Status Icons**: Visual feedback for quick status recognition
9. **History Cleanup**: Automatic to prevent memory growth
10. **Command Registry Integration**: Consistent with existing TUI patterns
