package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Task represents a todo item
type Task struct {
	ID                int       `json:"id"`
	Description       string    `json:"description"`
	Requester         string    `json:"requester"`
	CreatedAt         time.Time `json:"created_at"`
	CompletedAt       time.Time `json:"completed_at,omitempty"`
	EstimatedDuration int       `json:"estimated_duration"` // in minutes
	Priority          int       `json:"priority"`           // 1-5, where 1 is highest
	Status            string    `json:"status"`             // "pending", "in_progress", "completed"
	Position          int       `json:"position"`           // Position in the queue
}

// TaskList represents the list of tasks
type TaskList struct {
	Tasks  []Task `json:"tasks"`
	NextID int    `json:"next_id"`
}

// Global variables
var (
	tasksFile string
	taskList  TaskList
)

func init() {
	homeDir, _ := os.UserHomeDir()
	tasksFile = filepath.Join(homeDir, ".tasks.json")
	loadTasks()

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(completeCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(estimateCmd)
	rootCmd.AddCommand(queueInfoCmd)
	rootCmd.AddCommand(removeCmd)
}

// loadTasks loads tasks from the JSON file
func loadTasks() {
	if _, err := os.Stat(tasksFile); os.IsNotExist(err) {
		taskList = TaskList{
			Tasks:  []Task{},
			NextID: 1,
		}
		saveTasks()
		return
	}

	data, err := os.ReadFile(tasksFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading tasks file: %v\n", err)
		os.Exit(1)
	}

	if err := json.Unmarshal(data, &taskList); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing tasks file: %v\n", err)
		os.Exit(1)
	}
}

// saveTasks saves tasks to the JSON file
func saveTasks() {
	updateQueuePositions()

	data, err := json.MarshalIndent(taskList, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error serializing tasks: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(tasksFile, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing tasks file: %v\n", err)
		os.Exit(1)
	}
}

// updateQueuePositions recalculates queue positions based on priority and creation time
func updateQueuePositions() {
	var pendingTasks []Task

	// Get all pending tasks
	for _, task := range taskList.Tasks {
		if task.Status == "pending" {
			pendingTasks = append(pendingTasks, task)
		}
	}

	// Sort by priority (higher priority first) and then by creation time
	sort.Slice(pendingTasks, func(i, j int) bool {
		if pendingTasks[i].Priority != pendingTasks[j].Priority {
			return pendingTasks[i].Priority < pendingTasks[j].Priority // Lower number = higher priority
		}
		return pendingTasks[i].CreatedAt.Before(pendingTasks[j].CreatedAt)
	})

	// Update positions in the queue
	for i, task := range pendingTasks {
		for j := range taskList.Tasks {
			if taskList.Tasks[j].ID == task.ID {
				taskList.Tasks[j].Position = i + 1
				break
			}
		}
	}
}

// calculateEstimatedWaitTime calculates the estimated wait time for a task in minutes
func calculateEstimatedWaitTime(position int) int {
	var waitTime int

	for _, task := range taskList.Tasks {
		if task.Status == "pending" && task.Position < position {
			waitTime += task.EstimatedDuration
		}
	}

	return waitTime
}

// formatDuration formats minutes into a human-readable duration
func formatDuration(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%d minutes", minutes)
	} else if minutes < 1440 { // less than a day
		hours := minutes / 60
		mins := minutes % 60
		if mins == 0 {
			return fmt.Sprintf("%d hours", hours)
		}
		return fmt.Sprintf("%d hours, %d minutes", hours, mins)
	} else {
		days := minutes / 1440
		remainingMinutes := minutes % 1440
		hours := remainingMinutes / 60
		mins := remainingMinutes % 60

		result := fmt.Sprintf("%d days", days)
		if hours > 0 {
			result += fmt.Sprintf(", %d hours", hours)
		}
		if mins > 0 {
			result += fmt.Sprintf(", %d minutes", mins)
		}
		return result
	}
}

var rootCmd = &cobra.Command{
	Use:   "todo",
	Short: "A simple todo list manager with a queue system",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var addCmd = &cobra.Command{
	Use:   "add [description]",
	Short: "Add a new task to the queue",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		requester, _ := cmd.Flags().GetString("requester")
		priorityStr, _ := cmd.Flags().GetString("priority")
		durationStr, _ := cmd.Flags().GetString("duration")

		description := strings.Join(args, " ")

		priority := 3 // Default medium priority
		if p, err := strconv.Atoi(priorityStr); err == nil && p >= 1 && p <= 5 {
			priority = p
		}

		duration := 30 // Default 30 minutes
		if d, err := strconv.Atoi(durationStr); err == nil && d > 0 {
			duration = d
		}

		task := Task{
			ID:                taskList.NextID,
			Description:       description,
			Requester:         requester,
			CreatedAt:         time.Now(),
			Status:            "pending",
			Priority:          priority,
			EstimatedDuration: duration,
		}

		taskList.Tasks = append(taskList.Tasks, task)
		taskList.NextID++
		saveTasks()

		// Find the task's position in the queue
		var position int
		for _, t := range taskList.Tasks {
			if t.ID == task.ID {
				position = t.Position
				break
			}
		}

		waitTime := calculateEstimatedWaitTime(position)
		formattedWaitTime := formatDuration(waitTime)

		fmt.Printf("Added task #%d: %s\n", task.ID, task.Description)
		fmt.Printf("Queue position: %d\n", position)
		fmt.Printf("Estimated wait time: %s\n", formattedWaitTime)
	},
}

func init() {
	addCmd.Flags().StringP("requester", "r", "", "Person who requested this task")
	addCmd.Flags().StringP("priority", "p", "3", "Priority (1-5, where 1 is highest)")
	addCmd.Flags().StringP("duration", "d", "30", "Estimated duration in minutes")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tasks",
	Run: func(cmd *cobra.Command, args []string) {
		status, _ := cmd.Flags().GetString("status")

		fmt.Println("ID | Queue Pos | Priority |  Status  | Est. Duration | Requester | Description")
		fmt.Println("---|-----------|----------|----------|---------------|-----------|------------")

		for _, task := range taskList.Tasks {
			if status == "" || status == "all" || task.Status == status {
				posStr := "-"
				if task.Status == "pending" {
					posStr = fmt.Sprintf("%d", task.Position)
				}

				fmt.Printf("%2d | %9s | %8d | %8s | %13d | %9s | %s\n",
					task.ID, posStr, task.Priority, task.Status, task.EstimatedDuration, task.Requester, task.Description)
			}
		}
	},
}

func init() {
	listCmd.Flags().StringP("status", "s", "", "Filter by status (pending/in_progress/completed/all)")
}

var completeCmd = &cobra.Command{
	Use:   "complete [task_id]",
	Short: "Mark a task as completed",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid task ID: %s\n", args[0])
			os.Exit(1)
		}

		taskFound := false
		for i, task := range taskList.Tasks {
			if task.ID == id {
				taskList.Tasks[i].Status = "completed"
				taskList.Tasks[i].CompletedAt = time.Now()
				taskFound = true
				fmt.Printf("Marked task #%d as completed\n", id)
				break
			}
		}

		if !taskFound {
			fmt.Fprintf(os.Stderr, "Task #%d not found\n", id)
			os.Exit(1)
		}

		saveTasks()
	},
}

var startCmd = &cobra.Command{
	Use:   "start [task_id]",
	Short: "Mark a task as in progress",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid task ID: %s\n", args[0])
			os.Exit(1)
		}

		taskFound := false
		for i, task := range taskList.Tasks {
			if task.ID == id {
				taskList.Tasks[i].Status = "in_progress"
				taskFound = true
				fmt.Printf("Started working on task #%d\n", id)
				break
			}
		}

		if !taskFound {
			fmt.Fprintf(os.Stderr, "Task #%d not found\n", id)
			os.Exit(1)
		}

		saveTasks()
	},
}

var estimateCmd = &cobra.Command{
	Use:   "estimate [task_id] [duration]",
	Short: "Update the estimated duration for a task (in minutes)",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid task ID: %s\n", args[0])
			os.Exit(1)
		}

		duration, err := strconv.Atoi(args[1])
		if err != nil || duration <= 0 {
			fmt.Fprintf(os.Stderr, "Invalid duration: %s\n", args[1])
			os.Exit(1)
		}

		taskFound := false
		for i, task := range taskList.Tasks {
			if task.ID == id {
				taskList.Tasks[i].EstimatedDuration = duration
				taskFound = true
				fmt.Printf("Updated estimated duration for task #%d to %d minutes\n", id, duration)
				break
			}
		}

		if !taskFound {
			fmt.Fprintf(os.Stderr, "Task #%d not found\n", id)
			os.Exit(1)
		}

		saveTasks()
	},
}

var removeCmd = &cobra.Command{
	Use:     "remove [task_id]",
	Short:   "Remove a task from the list",
	Aliases: []string{"cancel", "delete"},
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid task ID: %s\n", args[0])
			os.Exit(1)
		}

		taskFound := false
		taskIndex := -1
		var taskDescription string

		for i, task := range taskList.Tasks {
			if task.ID == id {
				taskFound = true
				taskIndex = i
				taskDescription = task.Description
				break
			}
		}

		if !taskFound {
			fmt.Fprintf(os.Stderr, "Task #%d not found\n", id)
			os.Exit(1)
		}

		// Remove the task
		taskList.Tasks = append(taskList.Tasks[:taskIndex], taskList.Tasks[taskIndex+1:]...)
		saveTasks()

		fmt.Printf("Removed task #%d: %s\n", id, taskDescription)
		fmt.Println("Queue positions have been updated")
	},
}

var queueInfoCmd = &cobra.Command{
	Use:   "queue",
	Short: "Show current queue information",
	Run: func(cmd *cobra.Command, args []string) {
		pendingTasks := 0
		inProgressTasks := 0
		completedTasks := 0
		totalEstimatedTime := 0

		for _, task := range taskList.Tasks {
			switch task.Status {
			case "pending":
				pendingTasks++
				totalEstimatedTime += task.EstimatedDuration
			case "in_progress":
				inProgressTasks++
			case "completed":
				completedTasks++
			}
		}

		fmt.Println("Queue Status Summary:")
		fmt.Printf("- Pending tasks: %d\n", pendingTasks)
		fmt.Printf("- In progress tasks: %d\n", inProgressTasks)
		fmt.Printf("- Completed tasks: %d\n", completedTasks)
		fmt.Printf("- Total estimated wait time: %s\n", formatDuration(totalEstimatedTime))

		if pendingTasks > 0 {
			fmt.Println("\nCurrent Queue:")
			fmt.Println("Position | ID | Priority | Est. Duration | Requester | Description")
			fmt.Println("---------|----|---------:|---------------|-----------|------------")

			for _, task := range taskList.Tasks {
				if task.Status == "pending" {
					waitTime := calculateEstimatedWaitTime(task.Position)
					fmt.Println("waitTime", waitTime)
					fmt.Printf("%8d | %2d | %8d | %13s | %9s | %s\n",
						task.Position, task.ID, task.Priority,
						formatDuration(task.EstimatedDuration),
						task.Requester, task.Description)
				}
			}
		}
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
