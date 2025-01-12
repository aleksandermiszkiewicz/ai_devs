package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"log"
	"os"
)

func main() {
	ctx := context.Background()
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Fatalf("could not load env variables %v", err)
	}

	paths := []string{
		"../../images/image01.png",
		"../../images/image02.png",
		"../../images/image03.png",
		"../../images/image04.png",
	}

	openaiClient := openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY")))

	systemMsg := prepareSystemMessage()
	messages := []openai.ChatCompletionMessageParamUnion{systemMsg}

	for _, path := range paths {
		msg, err := prepareImageUserMessage(path)
		if err != nil {
			log.Fatalln(err)
		}
		messages = append(messages, *msg)
	}

	answer, err := callModelForAnswer(ctx, openaiClient, messages)
	if err != nil {
		log.Fatalln("error while calling openai", err)
	}
	log.Printf("final response is: %s", answer)
}

func prepareSystemMessage() openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role: openai.F(openai.ChatCompletionMessageParamRole("system")),
		Content: openai.F[interface{}]("You are an expert at Polish geography, topography, architecture and history.\n " +
			"You are looking at different parts of a map of a city in Poland. It's not \"Toruń\" and it's not \"Kalisz\" and it's not \"Bydgoszcz\".\n " +
			"There used to be \"spichlerze i twierdze\" in the city. Some maps contain street numbers - pay special attention to them. \n  " +
			"Warning: one of the parts shows a map of a different city.\n Based on the geographical features, street layouts, and any visible landmarks, can you identify which city this is?  " +
			"Please provide your reasoning."),
	}
}

func prepareImageUserMessage(imagePath string) (*openai.ChatCompletionMessageParam, error) {
	fileContent, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("something went wrong while reading file %s", imagePath)
	}
	encoded := base64.StdEncoding.EncodeToString(fileContent)
	iamgeUrl := openai.ChatCompletionContentPartImageImageURLParam{
		URL:    openai.F[string](fmt.Sprintf("data:image/png;base64,%s", encoded)),
		Detail: openai.F[openai.ChatCompletionContentPartImageImageURLDetail](openai.ChatCompletionContentPartImageImageURLDetailHigh),
	}
	return &openai.ChatCompletionMessageParam{
		Role: openai.F(openai.ChatCompletionMessageParamRole("user")),
		Content: openai.F[interface{}]([]openai.ChatCompletionContentPartImageParam{openai.ChatCompletionContentPartImageParam{
			Type:     openai.F[openai.ChatCompletionContentPartImageType](openai.ChatCompletionContentPartImageType(openai.ChatCompletionContentPartTypeImageURL)),
			ImageURL: openai.F[openai.ChatCompletionContentPartImageImageURLParam](iamgeUrl),
		}}),
	}, nil
}

func callModelForAnswer(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessageParamUnion) (string, error) {
	chatCompletion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages:    openai.F(messages),
		Model:       openai.F(openai.ChatModelGPT4o),
		Temperature: openai.F[float64](0),
	})

	if err != nil {
		return "", err
	}
	answer := chatCompletion.Choices[0].Message.Content
	log.Printf("answer -> %s", answer)
	return answer, nil
}
