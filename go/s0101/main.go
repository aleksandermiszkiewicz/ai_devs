package main

import (
	"context"
	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func main() {
	ctx := context.Background()
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Fatalf("could not load env variables %v", err)
	}

	openaiClient := openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY")))
	client := http.Client{}

	host := os.Getenv("HOST")
	resp, err := client.Get(host)

	if err != nil {
		log.Fatalln("could not fetch resources from hot", err)
	}

	if resp.StatusCode == http.StatusOK {
		question := extractQuestion(resp.Body)
		answer := callModelForAnswer(ctx, openaiClient, question)
		values := url.Values{}
		values.Add("username", os.Getenv("AGENT_USER"))
		values.Add("password", os.Getenv("AGENT_PASSWORD"))
		values.Add("answer", answer)
		finalResp, err := client.PostForm(host, values)
		if err != nil {
			log.Fatalln("something went wrong while logging to the system")
		}
		if finalResp.StatusCode == http.StatusOK {
			log.Print("logging with success")
			result, err := io.ReadAll(finalResp.Body)
			if err != nil {
				log.Fatalf("could not read body %v", err)
			}
			log.Printf(string(result))
		}
	}
}

func extractQuestion(closer io.ReadCloser) string {
	doc, err := goquery.NewDocumentFromReader(closer)
	if err != nil {
		log.Fatalf("could not load html %v", err)
	}

	questionParagraph := doc.Find("p#human-question")

	html, err := questionParagraph.Html()
	if err != nil {
		log.Fatalf("error extracting paragraph from HTML: %v", err)
	}

	parts := strings.Split(html, "<br/>")
	if len(parts) < 2 {
		log.Fatalf("Unexpected format in human-question paragraph")
	}
	question := strings.TrimSpace(parts[1])
	log.Printf("extracted question %s", question)
	return question
}

func prepareSystemMessage() openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role:    openai.F(openai.ChatCompletionMessageParamRole("system")),
		Content: openai.F[interface{}]("You will receive the question. You need to answer as short as possible, the best answer is 1 word if possible."),
	}
}

func prepareUserMessage(question string) openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role:    openai.F(openai.ChatCompletionMessageParamRole("user")),
		Content: openai.F[interface{}](question),
	}
}

func callModelForAnswer(ctx context.Context, client *openai.Client, question string) string {
	chatCompletion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{prepareSystemMessage(), prepareUserMessage(question)}),
		Model:    openai.F(openai.ChatModelGPT4oMini),
	})

	if err != nil {
		log.Fatalln("error while calling openai", err)
	}
	answer := chatCompletion.Choices[0].Message.Content
	log.Printf("answer -> %s", answer)
	return answer
}
