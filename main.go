package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// HistoryEntry represents a single command from the Zsh history.
type HistoryEntry struct {
	Timestamp time.Time
	Command   string
}

// main is the entry point of the program.
func main() {
	// 1. Find the Zsh history file.
	historyFile := getZshHistoryFile()
	if historyFile == "" {
		fmt.Fprintf(os.Stderr, "Error: Zsh history file not found.\n")
		fmt.Fprintf(os.Stderr, "Historik only supports zsh history and requires a non-empty HISTFILE environment variable or a default .zsh_history file.\n")
		os.Exit(1)
	}

	// 2. Parse the history file into a slice of HistoryEntry structs.
	entries, err := parseZshHistory(historyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not read history file: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "History is empty\n")
		os.Exit(1)
	}

	// 3. Remove duplicate commands, keeping only the most recent one.
	uniqueEntries := removeDuplicates(entries)

	// 4. Use FZF to allow the user to select a command from the unique history.
	selectedCommand, err := searchWithFZF(uniqueEntries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 5. If a command was selected, execute it.
	if selectedCommand != "" {
		executeCommand(selectedCommand)
	}
}

// getZshHistoryFile locates the Zsh history file, checking HISTFILE first.
func getZshHistoryFile() string {
	// First check the HISTFILE environment variable.
	if histfile := os.Getenv("HISTFILE"); histfile != "" {
		if _, err := os.Stat(histfile); err == nil {
			return histfile
		}
	}

	// If HISTFILE is not set or doesn't exist, check the default location.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	defaultHistFile := filepath.Join(homeDir, ".zsh_history")
	if _, err := os.Stat(defaultHistFile); err == nil {
		return defaultHistFile
	}

	return ""
}

// parseZshHistory parses the Zsh history file, which can contain multi-line commands.
func parseZshHistory(filename string) ([]HistoryEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []HistoryEntry
	scanner := bufio.NewScanner(file)
	// Regex to match the Zsh extended history format: ": timestamp:elapsed;command"
	extendedHistoryRegex := regexp.MustCompile(`^: (\d+):\d+;(.*)$`)

	var currentEntry *HistoryEntry

	for scanner.Scan() {
		line := scanner.Text()

		// Attempt to match a new history entry with a timestamp.
		if matches := extendedHistoryRegex.FindStringSubmatch(line); matches != nil {
			// If a new entry is found, and there's a previous entry to save, save it.
			if currentEntry != nil {
				// Skip empty commands and the historik command itself.
				if currentEntry.Command != "" && !strings.HasPrefix(currentEntry.Command, "historik") {
					entries = append(entries, *currentEntry)
				}
			}

			// Start a new history entry.
			timestamp, err := strconv.ParseInt(matches[1], 10, 64)
			var entryTimestamp time.Time
			if err == nil {
				entryTimestamp = time.Unix(timestamp, 0)
			}
			currentEntry = &HistoryEntry{
				Timestamp: entryTimestamp,
				Command:   strings.TrimSpace(matches[2]),
			}
		} else {
			// This line is a continuation of the previous command.
			if currentEntry != nil {
				currentEntry.Command += "\n" + line
			}
		}
	}

	// Process the final entry after the loop finishes.
	if currentEntry != nil {
		if currentEntry.Command != "" && !strings.HasPrefix(currentEntry.Command, "historik") {
			entries = append(entries, *currentEntry)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// removeDuplicates removes duplicate commands, keeping the most recent one.
func removeDuplicates(entries []HistoryEntry) []HistoryEntry {
	seen := make(map[string]HistoryEntry)

	// Iterate backwards to ensure the most recent entry is kept.
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		// Use the command as the key.
		if _, exists := seen[entry.Command]; !exists {
			seen[entry.Command] = entry
		}
	}

	// Convert the map back to a slice.
	unique := make([]HistoryEntry, 0, len(seen))
	for _, entry := range seen {
		unique = append(unique, entry)
	}

	// Sort the slice by timestamp in reverse order (newest first).
	sort.Slice(unique, func(i, j int) bool {
		// Place entries with zero timestamps at the end.
		if unique[i].Timestamp.IsZero() {
			return false
		}
		if unique[j].Timestamp.IsZero() {
			return true
		}
		return unique[i].Timestamp.After(unique[j].Timestamp)
	})

	return unique
}

// searchWithFZF pipes the history to fzf and returns the user's selection.
func searchWithFZF(entries []HistoryEntry) (string, error) {
	// Check if FZF is installed.
	if _, err := exec.LookPath("fzf"); err != nil {
		return "", fmt.Errorf("fzf is not installed. Please install it to use this tool")
	}

	// Set up the FZF command with a custom prompt, border, and key bindings.
	cmd := exec.Command("fzf",
		"--height=40%",
		"--reverse",
		"--border",
		"--prompt=Historik > ",
		"--bind=ctrl-r:toggle-sort",
		"--header=CTRL-R: toggle sort, ESC: quit",
	)
	cmd.Stderr = os.Stderr

	// Create a pipe to write the history to FZF's standard input.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe for fzf: %v", err)
	}
	defer stdin.Close()

	// Capture the output of FZF.
	output, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe for fzf: %v", err)
	}
	defer output.Close()

	// Start the FZF process.
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start fzf process: %v", err)
	}

	// Write each history command to FZF's stdin.
	for _, entry := range entries {
		io.WriteString(stdin, entry.Command+"\n")
	}

	// Close the stdin pipe to signal the end of input to FZF.
	stdin.Close()

	// Read the selected command from FZF's stdout.
	selected, err := io.ReadAll(output)
	if err != nil {
		return "", fmt.Errorf("failed to read fzf output: %v", err)
	}

	// Wait for the FZF command to finish.
	if err := cmd.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// Exit code 130 is returned when the user cancels with ESC.
			if exitError.ExitCode() == 130 {
				return "", nil
			}
		}
		return "", fmt.Errorf("fzf failed: %v", err)
	}

	return strings.TrimSpace(string(selected)), nil
}

// executeCommand executes the selected command in a new Zsh shell.
func executeCommand(command string) {
	// Use `zsh -c` to execute the command, which respects shell features like aliases.
	cmd := exec.Command("zsh", "-c", command)

	// Connect all I/O to the current process. This allows the user to interact with the command.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the command and wait for it to complete.
	err := cmd.Run()
	if err != nil {
		// Propagate the exit code if the command fails.
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		os.Exit(1)
	}
}
