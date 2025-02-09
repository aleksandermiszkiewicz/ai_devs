package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Instruction struct {
	Data string `json:"instruction"`
}

type Response struct {
	Description string `json:"description"`
}

func main() {
	ctx := context.Background()
	err := godotenv.Load("../../.env")

	if err != nil {
		log.Fatalf("could not load env variables %v", err)
	}

	openaiApiKey := os.Getenv("OPENAI_API_KEY")

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		webhookHandler(w, r, openaiApiKey, ctx)
	})

	addr := ":3002"
	log.Printf("Starting server on %s\n", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("could not start server: %v\n", err)
	}
}

func webhookHandler(w http.ResponseWriter, r *http.Request, apikey string, ctx context.Context) {
	if r.Method != http.MethodPost {
		log.Println("error: received non-post request")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	bodyBytes, _ := io.ReadAll(r.Body)
	content := string(bodyBytes)
	log.Println(fmt.Sprintf("Receive request -> %s | %s", r.Method, content))
	if strings.Contains(content, "{{") {
		log.Println(fmt.Sprintf("FLAG!!!! -> %s", content))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("thanks"))
		return
	}
	var data Instruction
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	resp, err := prepareResponse(ctx, apikey, data.Data)
	if err != nil {
		http.Error(w, "somthing went wrong while calling external source", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
	}
}

func prepareResponse(ctx context.Context, apikey string, instruction string) (*Response, error) {
	messages := []openai.ChatCompletionMessageParamUnion{prepareSystemMessage(), prepareUserMessage(instruction)}
	openaiClient := openai.NewClient(option.WithAPIKey(apikey))
	resp, err := callModel(ctx, openaiClient, messages)
	if err != nil {
		return nil, err
	}
	log.Println(fmt.Sprintf("model response -> %s", resp))
	return &Response{Description: resp}, nil

}

func callModel(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessageParamUnion) (string, error) {
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F(messages),
		Model:    openai.F(openai.ChatModelGPT4oMini),
	})
	if err != nil {
		log.Fatalf("Error while sending message: %v", err)
		return "", err
	}
	answer := resp.Choices[0].Message.Content
	return answer, nil
}

func prepareSystemMessage() openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role: openai.F(openai.ChatCompletionMessageParamRole("system")),
		Content: openai.F[interface{}]("" +
			"Jesteś dronem który lata po mapie z 16 polami - mapa 4 x 4. Mapa po której się poruszasz została opisana za pomocą HTML i przedstawiona poniżej:\n " +
			mapSetup() +
			"Otrzymujesz instrukcje które będą opisywać jak powinieneś się przemieszczać po mapie. Instrukcja zawierać będzie również pytanie, ktore będzie wymagać od Ciebie podania informacji gdzie obecnie się znajdujesz po wyokonaniu instrukcji.\n " +
			"Odpowiedź powinna uwzględniać TYLKO opis pola z mapy np. trawa, dwa drzewa, jaskinia itp.\n " +
			"Twoim punktem startowym jest pole z id=1 nazwane 'punkt startowy'\n " +
			"Kilka przykładów:\n " +
			"1. Instrukcja: 'S\\u0142uchaj kolego. Lecimy na maksa w prawo, a p\\u00f3\\u017aniej ile wlezie w d\\u00f3\\u0142. Co tam widzisz?'. Odpowiedź: jaskinia.\n " +
			"2. Instrukcja: 'Lecimy kolego teraz na sam d\\u00f3\\u0142 mapy, a p\\u00f3\\u017aniej ile tylko mo\\u017cemy polecimy w prawo. Teraz ma\\u0142a korekta o jedno pole do g\\u00f3ry. Co my tam mamy?'. Odpowiedź: dwa drzewa.\n " +
			"3. Instrukcja: 'Dobra. To co? zaczynamy? Odpalam silniki. Czas na kolejny lot. Jeste\\u015b moimi oczami. Lecimy w d\\u00f3\\u0142, albo nie! nie! czekaaaaj. Polecimy wiem jak. W prawo i dopiero teraz w d\\u00f3\\u0142. Tak b\\u0119dzie OK. Co widzisz?'. Odpowiedź: młyn" +
			"4. Instrukcja: 'Polecimy na sam d\\u00f3\\u0142 mapy, a p\\u00f3\\u017aniej o dwa pola w prawo. Co tam jest?'. Odpowiedź: samochód",
		),
	}
}

func prepareUserMessage(question string) openai.ChatCompletionMessageParam {
	return openai.ChatCompletionMessageParam{
		Role:    openai.F(openai.ChatCompletionMessageParamRole("user")),
		Content: openai.F[interface{}](question),
	}
}

func mapSetup() string {
	return "<table>\n" +
		"    <tr><td id=\"1\">punkt startowy</td><td id=\"2\">trawa</td><td id=\"3\">drzewo</td><td id=\"4\">dom</td></tr>\n" +
		"    <tr><td id=\"5\">trawa</td><td id=\"6\">młyn</td><td id=\"7\">trawa</td><td id=\"8\">trawa</td></tr>\n" +
		"    <tr><td id=\"9\">trawa</td><td id=\"10\">trawa</td><td id=\"11\">skały</td><td id=\"12\">dwa drzewa</td></tr>\n" +
		"    <tr><td id=\"13\">skały</td><td id=\"14\">skały</td><td id=\"15\">samochód</td><td id=\"16\">jaskinia</td></tr>\n" +
		"</table>\n"
}
