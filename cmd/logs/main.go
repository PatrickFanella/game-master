// Command logs tails a game-master JSONL log file with colorized output and filtering.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/PatrickFanella/game-master/internal/logging"
)

func main() {
	filePath := flag.String("file", ".logs/game-master.jsonl", "path to JSONL log file")
	levelStr := flag.String("level", "debug", "minimum log level: debug, info, warn, error")
	serviceCSV := flag.String("service", "", "comma-separated service filter (empty = all)")
	history := flag.Int("history", 0, "number of historical lines to show before tailing (0 = skip to end)")
	flag.Parse()

	minLevel := parseLevel(*levelStr)
	services := parseServices(*serviceCSV)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	file := waitForFile(ctx, *filePath)
	if file == nil {
		return // context cancelled while waiting
	}
	defer file.Close()

	if *history > 0 {
		printHistory(file, *history, minLevel, services)
	} else {
		// Seek to end for live tail only.
		if _, err := file.Seek(0, io.SeekEnd); err != nil {
			fmt.Fprintf(os.Stderr, "seek: %v\n", err)
			os.Exit(1)
		}
	}

	tail(ctx, file, minLevel, services)
}

// waitForFile opens the file, polling every 500ms if it doesn't exist yet.
func waitForFile(ctx context.Context, path string) *os.File {
	f, err := os.Open(path)
	if err == nil {
		return f
	}
	if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", path, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Waiting for log file...\n")
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			f, err = os.Open(path)
			if err == nil {
				return f
			}
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "open %s: %v\n", path, err)
				os.Exit(1)
			}
		}
	}
}

// printHistory seeks backwards from end-of-file to find the last N lines, then prints them.
func printHistory(file *os.File, n int, minLevel slog.Level, services map[string]bool) {
	// Read entire file to find last N lines. For simplicity and correctness
	// with variable-width UTF-8 JSONL, we scan forward and keep a ring of lines.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		fmt.Fprintf(os.Stderr, "seek: %v\n", err)
		return
	}

	// Collect all lines, then take last N that pass filters.
	var matched []logging.LogEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		entry, err := logging.ParseJSONLEntry(line)
		if err != nil {
			continue
		}
		if !passesFilter(entry, minLevel, services) {
			continue
		}
		matched = append(matched, entry)
	}

	// Print last N.
	start := 0
	if len(matched) > n {
		start = len(matched) - n
	}
	for _, entry := range matched[start:] {
		fmt.Print(logging.FormatColorLine(entry))
	}

	// Leave file position at end for subsequent tailing.
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		fmt.Fprintf(os.Stderr, "seek: %v\n", err)
	}
}

// tail polls the file for new JSONL lines and prints matching entries.
func tail(ctx context.Context, file *os.File, minLevel slog.Level, services map[string]bool) {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for {
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			entry, err := logging.ParseJSONLEntry(line)
			if err != nil {
				continue
			}
			if !passesFilter(entry, minLevel, services) {
				continue
			}
			fmt.Print(logging.FormatColorLine(entry))
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelDebug
	}
}

func parseServices(csv string) map[string]bool {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return nil
	}
	m := make(map[string]bool)
	for _, s := range strings.Split(csv, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			m[s] = true
		}
	}
	return m
}

func passesFilter(entry logging.LogEntry, minLevel slog.Level, services map[string]bool) bool {
	if entry.Level < minLevel {
		return false
	}
	if len(services) > 0 && !services[entry.Service] {
		return false
	}
	return true
}
