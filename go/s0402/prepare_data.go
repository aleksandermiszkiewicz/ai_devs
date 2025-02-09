package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"slices"
)

const rootDir = "../../lab_data"

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatPrompt struct {
	Messages []ChatMessage `json:"messages"`
}

func main() {
	incorrect := ReadFile(fmt.Sprintf("%s/%s", rootDir, "incorrect.txt"))
	correct := ReadFile(fmt.Sprintf("%s/%s", rootDir, "correct.txt"))
	var prompts []ChatPrompt
	prompts = slices.Concat(prompts, prepareTrainingData(incorrect, false, 100))
	prompts = slices.Concat(prompts, prepareTrainingData(correct, true, 100))

	rand.Shuffle(len(prompts), func(i, j int) {
		prompts[i], prompts[j] = prompts[j], prompts[i]
	})

	file, err := os.Create("data.jsonl")
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)

	for _, prompt := range prompts {
		if err := encoder.Encode(prompt); err != nil {
			log.Fatalf("Error encoding prompt: %v", err)
		}
	}
	log.Println("JSONL file created successfully.")
}

func prepareTrainingData(content []string, correct bool, threshold int) []ChatPrompt {
	var prompts []ChatPrompt
	for i := 0; i <= threshold && i < len(content); i++ {
		prompts = append(prompts, ChatPrompt{
			Messages: []ChatMessage{
				{Role: "system", Content: "Classify result"},
				{Role: "user", Content: content[i]},
				{Role: "assistant", Content: map[bool]string{true: "Y", false: "N"}[correct]},
			},
		})
	}
	return prompts
}
