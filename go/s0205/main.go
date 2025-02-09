package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"golang.org/x/net/html"
	"google.golang.org/api/option"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const modelType = "gemini-2.0-flash-exp"

type FinalAnswer struct {
	Task   string            `json:"task"`
	APIKey string            `json:"apikey"`
	Answer map[string]string `json:"answer"`
}

func main() {
	ctx := context.Background()
	err := godotenv.Load("../../.env")

	if err != nil {
		log.Fatalf("could not load env variables %v", err)
	}

	host := os.Getenv("CENTRALA_HOST")
	aiDevsApiKey := os.Getenv("AI_DEVS_API_KEY")
	geminiApiKey := os.Getenv("GEMINI_API_KEY")

	doIndexing(host, "dane/arxiv-draft.html")

	questions := fetchQuestions(host, aiDevsApiKey)

	promptMessages := []genai.Part{}
	promptMessages = append(promptMessages, systemPrompt())
	promptMessages = slices.Concat(promptMessages, prepareIndexedContent("indexed.md"))
	promptMessages = slices.Concat(promptMessages, prepareMessages("downloaded_images"))
	promptMessages = slices.Concat(promptMessages, prepareMessages("downloaded_audio"))

	client, err := genai.NewClient(ctx, option.WithAPIKey(geminiApiKey))
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}

	answers := make(map[string]string)
	for id, question := range questions {
		log.Printf("calling for answer on question %s", question)
		message := genai.Text(fmt.Sprintf("Pytanie: %s", question))
		resp, err := callModel(append(promptMessages, message), ctx, client)
		if err != nil {
			log.Fatalln("error: something went wrong while calling response", err)
		}
		log.Printf("answer: %s", resp)
		answers[id] = resp
		time.Sleep(5 * time.Second)
	}
	sendResult(host, aiDevsApiKey, answers)
}

func doIndexing(host string, path string) {
	resp, err := http.Get(fmt.Sprintf("%s/%s", host, path))
	if err != nil {
		log.Fatalf("error: getting article failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Fatalf("error: status %d %s", resp.StatusCode, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatalf("error: html parsing failed: %v", err)
	}

	outFile, err := os.Create("indexed.md")
	if err != nil {
		log.Fatalf("error: failed while createing indexed.md file: %v", err)
	}
	defer outFile.Close()

	fmt.Fprintln(outFile, "# Indeksowany artykuł profesora Maja\n")

	doTextIndexing(outFile, doc)
	doImagesIndexing(host, outFile, doc)
	doAudioIndexing(host, outFile, doc, err)
	fmt.Println("indexed.md file creation finished")
}

func doAudioIndexing(host string, outFile *os.File, doc *goquery.Document, err error) {
	fmt.Fprintln(outFile, "\n## Dźwięki\n")
	doc.Find("audio").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists || src == "" {
			s.Find("source").Each(func(j int, source *goquery.Selection) {
				if val, ok := source.Attr("src"); ok && src == "" {
					src = val
				}
			})
		}
		name := src[2:]
		src = fmt.Sprintf("%s/dane/%s", host, src)
		context := getParentContext(s)
		err = downloadFile(src, "downloaded_audio", name)
		if err != nil {
			log.Printf("error: could not fetch audio file %s: %v", src, err)
		} else {
			log.Printf("file saved: %s", filepath.Join("downloaded_audio", name))
		}

		fmt.Fprintf(outFile, "- Dźwięk %d: src='%s', name='%s' Kontekst: %s\n", i+1, src, name, context)
	})
}

func doImagesIndexing(host string, outFile *os.File, doc *goquery.Document) {
	fmt.Fprintln(outFile, "\n## Obrazy\n")
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		alt, _ := s.Attr("alt")
		name := src[2:]
		src = fmt.Sprintf("%s/dane/%s", host, src)
		context := getParentContext(s)

		err := downloadFile(src, "downloaded_images", name)
		if err != nil {
			log.Printf("error: could not fetch image %s: %v", src, err)
		} else {
			log.Printf("file saved %s", filepath.Join("downloaded_images", name))
		}

		fmt.Fprintf(outFile, "- Obraz %d: src='%s', alt='%s', name='%s' Kontekst: %s\n", i+1, src, alt, name, context)
	})
}

func doTextIndexing(outFile *os.File, doc *goquery.Document) {
	fmt.Fprintln(outFile, "## Treść tekstowa\n")
	doc.Find("p").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if text != "" {
			fmt.Fprintf(outFile, "- %s\n", text)
		}
	})
}

func downloadFile(fileURL, destDir, fileName string) error {
	err := os.MkdirAll(destDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("error: could not create directory %s: %v", destDir, err)
	}

	resp, err := http.Get(fileURL)
	if err != nil {
		return fmt.Errorf("error: could not download file %s: %v", fileURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error: file download failed -> %d | %s", resp.StatusCode, fileURL)
	}

	destPath := filepath.Join(destDir, fileName)
	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("error: %s: %v", destPath, err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("error: saving faile failed %s: %v", destPath, err)
	}

	return nil
}

func getParentContext(s *goquery.Selection) string {
	parent := s.Parent()
	if parent.Length() == 0 {
		return "Brak elementu nadrzędnego"
	}

	tagName := "unknown"
	if len(parent.Nodes) > 0 && parent.Nodes[0].Type == html.ElementNode {
		tagName = parent.Nodes[0].Data
	}

	contextText := strings.TrimSpace(parent.Text())
	if len(contextText) > 100 {
		contextText = contextText[:100] + "..."
	}

	return fmt.Sprintf("<%s> - \"%s\"", tagName, contextText)
}

func sendResult(host string, apikey string, answers map[string]string) {
	client := http.Client{}

	payload, _ := json.Marshal(FinalAnswer{
		Task:   "arxiv",
		APIKey: apikey,
		Answer: answers,
	})

	fmt.Println(string(payload))

	resp, err := client.Post(fmt.Sprintf("%s/report", host), "application/json", bytes.NewBuffer(payload))

	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()

	bytesBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("calling endpoint failed | %v | %s", resp.StatusCode, string(bytesBody))
	}

	log.Printf("result correct!")
	log.Printf(string(bytesBody))
}

func fetchQuestions(host string, apikey string) map[string]string {
	client := http.Client{}
	resp, err := client.Get(fmt.Sprintf("%s/data/%s/arxiv.txt", host, apikey))
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Fatalln("error: fetching questions failed", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln("error: could not read response body", err)
	}

	questions := make(map[string]string)

	lines := strings.Split(string(bodyBytes), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			questions[key] = value
		}
	}

	for key, value := range questions {
		fmt.Printf("%s: %s\n", key, value)
	}

	return questions
}

func prepareIndexedContent(file string) []genai.Part {
	fileContent, err := os.ReadFile(file)
	if err != nil {
		log.Fatalln(fmt.Sprintf("error: could not read file %s", file), err)
	}
	return []genai.Part{genai.Text("This is indexed content."), genai.Text(fileContent)}
}

func prepareMessages(dir string) []genai.Part {
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	var requestContent []genai.Part

	for _, file := range files {
		fileContent, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, file.Name()))
		if err != nil {
			log.Fatalln("error: could not read file ", err)
		}
		if strings.Contains(file.Name(), ".png") {
			requestContent = append(requestContent, genai.Text(file.Name()), genai.ImageData("png", fileContent))
		} else if strings.Contains(file.Name(), ".mp3") {
			requestContent = append(requestContent, genai.Text(file.Name()), genai.Blob{MIMEType: "audio/mp3", Data: fileContent})
		} else {
			log.Printf("warning: unknown file type %s", file.Name())
		}
	}
	return requestContent
}

func callModel(requestContent []genai.Part, ctx context.Context, client *genai.Client) (string, error) {
	model := client.GenerativeModel(modelType)

	model.SetTemperature(0.5)
	model.SetTopK(40)
	model.SetTopP(0.95)
	model.SetMaxOutputTokens(8192)

	session := model.StartChat()
	session.History = []*genai.Content{}

	resp, err := session.SendMessage(ctx, requestContent...)
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
		return "", err
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		content := fmt.Sprintf("%v", part)
		return content, nil
	}
	return "", nil
}

func systemPrompt() genai.Text {
	return "Jesteś pomocnym asystentem. Otrzymasz rożny kontent pochodzący z zaindeksowaniej strony HTML. " +
		"Kontent zawiera zaindeksowaną strone HTML (plik indexed.md) a także obrazy (pliki .png) oraz ścieżki audito (pliki .mp3). " +
		"Plik indexed.md zawiera przechwycone materiały które muszą Ci posłużyć do odpowiedzenia na pytania które otrzymasz. " +
		"W celu udzielenie odpowiedzi na pytania, musisz wziąć pod uwagę plik indexed.md a także pliki .png oraz .mp3." +
		"Odpowiedź na pytanie powinna być krótka i zwięzła, bez dodatkowych znaków. Jeżeli jest to możliwe to odpowiedź powinna być w formie jednego wyrazu." +
		"Dodatkowe informacje które powinieneś uwzględnić to to że Rynek to nie miasto. A w pytanie o Owoc musisz podać nazwę owocu. W przypadku nazw własnych podaj nazwy w oryginalnym języku."
}
