// Package logger provides centralized logging for the VPN client
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
)

var (
	logFile   *os.File
	logMutex  sync.Mutex
	logPath   string
	listeners []func(string)
	listMutex sync.RWMutex
)

// Init initializes the logger
func Init() error {
	logMutex.Lock()
	defer logMutex.Unlock()

	logPath = filepath.Join(getLogDir(), "vpn.log")

	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	logFile = f

	// Redirect stderr to log file so panics are captured
	redirectStderr(f)

	return nil
}

// Close closes the log file
func Close() {
	logMutex.Lock()
	defer logMutex.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// AddListener adds a callback that receives log messages
func AddListener(fn func(string)) {
	listMutex.Lock()
	defer listMutex.Unlock()
	listeners = append(listeners, fn)
}

// Log writes a log message
func Log(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s", timestamp, message)

	// Write to file
	logMutex.Lock()
	if logFile != nil {
		logFile.WriteString(line + "\n")
		logFile.Sync()
	}
	logMutex.Unlock()

	// Notify listeners
	listMutex.RLock()
	for _, fn := range listeners {
		go fn(line)
	}
	listMutex.RUnlock()
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	Log("INFO: "+format, args...)
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	Log("ERROR: "+format, args...)
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	Log("DEBUG: "+format, args...)
}

// Warning logs a warning message
func Warning(format string, args ...interface{}) {
	Log("WARN: "+format, args...)
}

// Connection logs a connection event
func Connection(format string, args ...interface{}) {
	Log("CONN: "+format, args...)
}

// GetLogPath returns the path to the log file
func GetLogPath() string {
	return logPath
}

// Recover should be deferred at the top of every goroutine to catch panics.
// Usage: go func() { defer logger.Recover("myGoroutine"); ... }()
func Recover(name string) {
	if r := recover(); r != nil {
		stack := string(debug.Stack())
		msg := fmt.Sprintf("PANIC in %s: %v\n%s", name, r, stack)
		Error(msg)
		// Also write directly to file in case Log() is broken
		logMutex.Lock()
		if logFile != nil {
			logFile.WriteString(fmt.Sprintf("[%s] FATAL PANIC: %s\n",
				time.Now().Format("2006-01-02 15:04:05"), msg))
			logFile.Sync()
		}
		logMutex.Unlock()
	}
}

// SafeGo launches a goroutine with panic recovery.
func SafeGo(name string, fn func()) {
	go func() {
		defer Recover(name)
		fn()
	}()
}

// ReadLogs reads the log file contents
func ReadLogs() (string, error) {
	if logPath == "" {
		logPath = filepath.Join(getLogDir(), "vpn.log")
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ClearLogs clears the log file
func ClearLogs() error {
	logMutex.Lock()
	defer logMutex.Unlock()

	if logFile != nil {
		logFile.Close()
	}

	if err := os.WriteFile(logPath, []byte{}, 0644); err != nil {
		return err
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	logFile = f
	return nil
}
