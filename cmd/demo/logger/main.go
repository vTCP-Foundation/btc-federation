package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v2"
)

const (
	buildDir         = "build"
	binaryName       = "btc-federation"
	configFileName   = "conf.yaml"
	requiredMessages = `["BTC Federation Node starting with config", "Node started successfully", "Listening addresses"]`
)

type LoggingConfig struct {
	Level         string `yaml:"level"`
	Format        string `yaml:"format"`
	ConsoleOutput bool   `yaml:"console_output"`
	ConsoleColor  bool   `yaml:"console_color"`
	FileOutput    bool   `yaml:"file_output"`
	FileName      string `yaml:"file_name"`
	FileMaxSize   string `yaml:"file_max_size"`
}

type Config struct {
	Logging LoggingConfig `yaml:"logging"`
}

func main() {
	fmt.Println("=== BTC Federation Logger Demo Validation ===")
	fmt.Println()

	// Track validation results
	allChecksPass := true
	checkNumber := 1

	// a. Check if btc-federation binary exists in build directory
	fmt.Printf("%d. Checking if btc-federation binary exists in build directory...\n", checkNumber)
	checkNumber++

	binaryPath := filepath.Join(buildDir, binaryName)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		fmt.Printf("   ❌ FAIL: Binary %s not found in %s directory\n", binaryName, buildDir)
		allChecksPass = false
	} else {
		fmt.Printf("   ✅ PASS: Binary found at %s\n", binaryPath)
	}

	// b. Check if conf.yaml exists in build directory
	fmt.Printf("%d. Checking if conf.yaml exists in build directory...\n", checkNumber)
	checkNumber++

	configPath := filepath.Join(buildDir, configFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("   ❌ FAIL: Config file %s not found in %s directory\n", configFileName, buildDir)
		allChecksPass = false
		return
	} else {
		fmt.Printf("   ✅ PASS: Config file found at %s\n", configPath)
	}

	// c. Read log file name from configuration file
	fmt.Printf("%d. Reading log file name from configuration...\n", checkNumber)
	checkNumber++

	config, err := readConfig(configPath)
	if err != nil {
		fmt.Printf("   ❌ FAIL: Failed to read config: %v\n", err)
		allChecksPass = false
		return
	}

	logFileName := config.Logging.FileName
	if logFileName == "" {
		fmt.Printf("   ❌ FAIL: Log file name not specified in config\n")
		allChecksPass = false
		return
	}

	fmt.Printf("   ✅ PASS: Log file name: %s\n", logFileName)

	// d. Delete existing log file with name from step c
	fmt.Printf("%d. Cleaning up existing log file...\n", checkNumber)
	checkNumber++

	logFilePath := filepath.Join(buildDir, logFileName)
	if err := os.Remove(logFilePath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("   ⚠️  WARNING: Failed to remove existing log file: %v\n", err)
	} else {
		fmt.Printf("   ✅ PASS: Log file cleanup completed\n")
	}

	// e. Start btc-federation binary and wait exactly 5 seconds
	fmt.Printf("%d. Starting btc-federation binary and waiting 5 seconds...\n", checkNumber)
	checkNumber++

	// Get absolute path to binary
	absPath, err := filepath.Abs(binaryPath)
	if err != nil {
		fmt.Printf("   ❌ FAIL: Failed to get absolute path: %v\n", err)
		allChecksPass = false
		return
	}

	cmd := exec.Command(absPath)
	cmd.Dir = filepath.Dir(absPath) // Set working directory to build dir

	if err := cmd.Start(); err != nil {
		fmt.Printf("   ❌ FAIL: Failed to start binary: %v\n", err)
		allChecksPass = false
		return
	}

	fmt.Printf("   ✅ Binary started with PID: %d\n", cmd.Process.Pid)
	fmt.Printf("   ⏳ Waiting 5 seconds...\n")

	time.Sleep(5 * time.Second)

	// Kill the process
	if err := cmd.Process.Kill(); err != nil {
		fmt.Printf("   ⚠️  WARNING: Failed to kill process: %v\n", err)
	} else {
		fmt.Printf("   ✅ Process terminated\n")
	}

	// f. Verify log file was created with name from step c
	fmt.Printf("%d. Verifying log file was created...\n", checkNumber)
	checkNumber++

	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		fmt.Printf("   ❌ FAIL: Log file %s was not created\n", logFilePath)
		allChecksPass = false
		return
	} else {
		fmt.Printf("   ✅ PASS: Log file created at %s\n", logFilePath)
	}

	// g. Verify log file contains valid JSON format entries
	fmt.Printf("%d. Verifying log file contains valid JSON format...\n", checkNumber)
	checkNumber++

	jsonValid, jsonLines, err := validateJSONFormat(logFilePath)
	if err != nil {
		fmt.Printf("   ❌ FAIL: Error reading log file: %v\n", err)
		allChecksPass = false
		return
	}

	if !jsonValid {
		fmt.Printf("   ❌ FAIL: Log file does not contain valid JSON format\n")
		allChecksPass = false
	} else {
		fmt.Printf("   ✅ PASS: Log file contains %d valid JSON entries\n", len(jsonLines))
	}

	// h. Verify log file contains required messages
	fmt.Printf("%d. Verifying log file contains required messages...\n", checkNumber)
	checkNumber++

	requiredMsgs := []string{
		"BTC Federation Node starting with config",
		"Node started successfully",
		"Listening addresses",
	}

	foundMessages := make(map[string]bool)
	for _, line := range jsonLines {
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
			if message, ok := logEntry["message"]; ok {
				msgStr := fmt.Sprintf("%v", message)
				for _, required := range requiredMsgs {
					if strings.Contains(msgStr, required) {
						foundMessages[required] = true
					}
				}
			}
		}
	}

	allMessagesFound := true
	for _, required := range requiredMsgs {
		if foundMessages[required] {
			fmt.Printf("   ✅ Found: \"%s\"\n", required)
		} else {
			fmt.Printf("   ❌ Missing: \"%s\"\n", required)
			allMessagesFound = false
		}
	}

	if !allMessagesFound {
		allChecksPass = false
	}

	// i. Verify zero instances of fmt.Printf/fmt.Println/log.Printf/log.Fatalf remain in cmd/ and internal/
	fmt.Printf("%d. Verifying no legacy logging calls remain in source code...\n", checkNumber)
	checkNumber++

	legacyCalls, err := findLegacyLoggingCalls()
	if err != nil {
		fmt.Printf("   ❌ FAIL: Error scanning source code: %v\n", err)
		allChecksPass = false
	} else if len(legacyCalls) > 0 {
		fmt.Printf("   ❌ FAIL: Found %d legacy logging calls:\n", len(legacyCalls))
		for _, call := range legacyCalls {
			fmt.Printf("       %s\n", call)
		}
		allChecksPass = false
	} else {
		fmt.Printf("   ✅ PASS: No legacy logging calls found in cmd/ and internal/ directories\n")
	}

	// j. Final result
	fmt.Printf("\n=== FINAL RESULT ===\n")
	if allChecksPass {
		fmt.Printf("🎉 SUCCESS: All validation checks passed!\n")
		fmt.Printf("The logging system implementation is complete and working correctly.\n")
		os.Exit(0)
	} else {
		fmt.Printf("❌ FAILURE: One or more validation checks failed.\n")
		fmt.Printf("Please fix the issues above and run the demo again.\n")
		os.Exit(1)
	}
}

func readConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func validateJSONFormat(logFilePath string) (bool, []string, error) {
	file, err := os.Open(logFilePath)
	if err != nil {
		return false, nil, err
	}
	defer file.Close()

	var jsonLines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try to parse as JSON
		var jsonObj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &jsonObj); err != nil {
			return false, nil, fmt.Errorf("invalid JSON line: %s", line)
		}

		jsonLines = append(jsonLines, line)
	}

	if err := scanner.Err(); err != nil {
		return false, nil, err
	}

	return len(jsonLines) > 0, jsonLines, nil
}

func findLegacyLoggingCalls() ([]string, error) {
	var legacyCalls []string

	// Search patterns for legacy logging calls
	patterns := []string{
		"fmt.Printf",
		"fmt.Println",
		"log.Printf",
		"log.Fatalf",
	}

	// Directories to search
	searchDirs := []string{"cmd", "internal"}

	for _, dir := range searchDirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip directories and non-Go files
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}

			// Skip logger implementation itself, test files, demo modules, and utility programs
			if strings.Contains(path, "internal/logger/logger.go") ||
				strings.HasSuffix(path, "_test.go") ||
				strings.Contains(path, "cmd/demo/") ||
				strings.Contains(path, "cmd/utils/") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			lines := strings.Split(string(content), "\n")
			for lineNum, line := range lines {
				// Skip commented lines
				trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
				if strings.HasPrefix(trimmed, "//") {
					continue
				}

				// Check for legacy logging patterns
				for _, pattern := range patterns {
					if strings.Contains(line, pattern) {
						legacyCalls = append(legacyCalls, fmt.Sprintf("%s:%d: %s", path, lineNum+1, strings.TrimSpace(line)))
					}
				}
			}

			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return legacyCalls, nil
}
