package main

import (
	"bufio"
	"log"
	"os"
)

func ReadFile(filePath string) []string {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	lineNumber := 1
	var content []string
	for scanner.Scan() {
		line := scanner.Text()
		content = append(content, line)
		lineNumber++
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading file: %v", err)
	}
	return content
}
