package cli

import (
	"fmt"
	"os"
	"strings"

	"vcode/internal/taskgraph"
)

func stderr() *os.File { return os.Stderr }

func taskCommand(args []string) int {
	root := mustCurrentDir()
	store := taskgraph.NewStore(root)
	if len(args) == 0 || args[0] == "list" {
		return listTasks(store)
	}
	switch args[0] {
	case "show":
		if len(args) < 2 {
			fmt.Fprintln(stderr(), "usage: vcode task show <task-id>")
			return 2
		}
		return showTask(store, args[1])
	case "create":
		goal := strings.TrimSpace(strings.Join(args[1:], " "))
		if goal == "" {
			fmt.Fprintln(stderr(), "usage: vcode task create <goal>")
			return 2
		}
		t, err := store.Create(goal, root, nil)
		if err != nil {
			fmt.Fprintln(stderr(), "error:", err)
			return 1
		}
		fmt.Printf("created task %s\n", t.ID)
		return 0
	case "resume", "retry", "pause", "cancel":
		if len(args) < 2 {
			fmt.Fprintln(stderr(), "usage: vcode task resume|retry|pause|cancel <task-id> [node-id]")
			return 2
		}
		return changeTaskState(store, args[0], args[1], args[2:])
	default:
		fmt.Fprintln(stderr(), "usage: vcode task [list|show|create|resume|retry|pause|cancel]")
		return 2
	}
}

func changeTaskState(store *taskgraph.Store, action, id string, rest []string) int {
	t, err := store.Get(id)
	if err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	if action == "pause" || action == "cancel" {
		status := taskgraph.Blocked
		if action == "cancel" {
			status = taskgraph.Cancelled
		}
		if err := store.SetStatus(&t, status, "changed by operator"); err != nil {
			fmt.Fprintln(stderr(), "error:", err)
			return 1
		}
		fmt.Printf("%s %s\n", id, status)
		return 0
	}
	if err := store.RecoverInterrupted(&t); err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	if len(rest) > 0 {
		status := taskgraph.Ready
		if action == "retry" {
			status = taskgraph.Pending
		}
		if err := store.UpdateNode(&t, rest[0], status, "scheduled by operator"); err != nil {
			fmt.Fprintln(stderr(), "error:", err)
			return 1
		}
	}
	if err := store.SetStatus(&t, taskgraph.Ready, "scheduled by operator"); err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	fmt.Printf("%s ready\n", id)
	return 0
}

func listTasks(store *taskgraph.Store) int {
	tasks, err := store.List()
	if err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	if len(tasks) == 0 {
		fmt.Println("no tasks")
		return 0
	}
	for _, t := range tasks {
		fmt.Printf("%-28s %-12s %s\n", t.ID, t.Status, t.Goal)
	}
	return 0
}

func showTask(store *taskgraph.Store, id string) int {
	t, err := store.Get(id)
	if err != nil {
		fmt.Fprintln(stderr(), "error:", err)
		return 1
	}
	fmt.Printf("%s  %s\n%s\n", t.ID, t.Status, t.Goal)
	for _, n := range t.Nodes {
		fmt.Printf("  %-12s %-10s %-12s %s\n", n.ID, n.Role, n.Status, n.Title)
	}
	return 0
}
