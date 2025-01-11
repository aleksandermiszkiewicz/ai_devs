import sys

import glob
import logging
from datetime import datetime
from typing import List, Dict, Any
import json
import requests
import torch
from json import JSONEncoder
import os
from transformers import AutoModelForSpeechSeq2Seq, AutoProcessor, pipeline, Pipeline
from dotenv import load_dotenv
from pathlib import Path

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
    handlers=[
        logging.StreamHandler(sys.stdout)
    ]
)

class FinalAnswer:
    def __init__(self, task: str, apikey: str, answer: str):
        self.task = task
        self.apikey = apikey
        self.answer = answer

    def to_dict(self) -> Dict[str, Any]:
        return {
            "task": self.task,
            "apikey": self.apikey,
            "answer": self.answer
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "FinalAnswer":
        return cls(
            task=data.get("task"),
            apikey=data.get("apikey"),
            answer=data.get("answer")
        )

    def __repr__(self) -> str:
        return f"FinalAnswer(task={self.task!r}, apikey={self.apikey!r}, answer={self.answer!r})"



class Message:
    def __init__(self, role: str, content: str):
        self.role = role
        self.content = content

    def to_dict(self) -> Dict[str, Any]:
        return {
            "role": self.role,
            "content": self.content,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Message":
        return cls(
            role=data.get("role"),
            content=data.get("content"),
        )

    def __repr__(self) -> str:
        return f"Message(role={self.role!r}, content={self.content!r})"

class Request:
    def __init__(self, model: str, messages: List[Message], stream: bool):
        self.model = model
        self.messages = messages
        self.stream = stream

    def to_dict(self) -> Dict[str, Any]:
        return {
            "model": self.model,
            "messages": [msg.to_dict() for msg in self.messages],
            "stream": self.stream,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Request":
        return cls(
            model=data.get("model"),
            messages=[Message.from_dict(msg) for msg in data.get("messages")],
            stream=data.get("stream"),
        )

    def __repr__(self) -> str:
        return f"Request(model={self.model!r}, messages={self.messages!r}, stream={self.stream!r})"


class Response:
    def __init__(
            self,
            model: str,
            created_at: datetime,
            message: Message,
            done: bool,
            total_duration: int,
            load_duration: int,
            prompt_eval_count: int,
            prompt_eval_duration: int,
            eval_count: int,
            eval_duration: int,
    ):
        self.model = model
        self.created_at = created_at
        self.message = message
        self.done = done
        self.total_duration = total_duration
        self.load_duration = load_duration
        self.prompt_eval_count = prompt_eval_count
        self.prompt_eval_duration = prompt_eval_duration
        self.eval_count = eval_count
        self.eval_duration = eval_duration

    def to_dict(self) -> Dict[str, Any]:
        return {
            "model": self.model,
            "created_at": self.created_at,
            "message": self.message,
            "done": self.done,
            "total_duration": self.total_duration,
            "load_duration": self.load_duration,
            "prompt_eval_count": self.prompt_eval_count,
            "prompt_eval_duration": self.prompt_eval_duration,
            "eval_count": self.eval_count,
            "eval_duration": self.eval_duration,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Response":
        return cls(
            model=data.get("model"),
            created_at=data.get("created_at"),
            message=Message.from_dict(data.get("message")),
            done=data.get("done"),
            total_duration=data.get("total_duration"),
            load_duration=data.get("load_duration"),
            prompt_eval_count=data.get("prompt_eval_count"),
            prompt_eval_duration=data.get("prompt_eval_duration"),
            eval_count=data.get("eval_count"),
            eval_duration=data.get("eval_duration"),
        )

    def __repr__(self) -> str:
        return (f"Request(model={self.model!r}, "
                f"created_at={self.created_at!r}, "
                f"message={self.message!r}, "
                f"done={self.done!r}, "
                f"total_duration={self.total_duration!r}, "
                f"load_duration={self.load_duration!r}, "
                f"prompt_eval_count={self.prompt_eval_count!r}, "
                f"eval_count={self.eval_count!r}, "
                f"eval_duration={self.eval_duration!r})")

class Encoder(JSONEncoder):
    def default(self, o):
        if hasattr(o, "to_dict"):
            return o.to_dict()
        return super().default(o)


speach_to_text_model_id = "openai/whisper-large-v3-turbo"
llm_model = "SpeakLeash/bielik-11b-v2.2-instruct:Q4_K_M"


def get_list_of_records() -> list[str]:
    files = glob.glob("../../records/*")
    logging.info(f"files to transcript: {files}")
    return files

def prepare_pipeline() -> Pipeline :
    device = "cuda:0" if torch.cuda.is_available() else "cpu"
    torch_dtype = torch.float16 if torch.cuda.is_available() else torch.float32
    model = AutoModelForSpeechSeq2Seq.from_pretrained(speach_to_text_model_id, torch_dtype=torch_dtype, low_cpu_mem_usage=True, use_safetensors=True)
    model.to(device)

    processor = AutoProcessor.from_pretrained(speach_to_text_model_id)

    pipe = pipeline(
        "automatic-speech-recognition",
        model=model,
        tokenizer=processor.tokenizer,
        feature_extractor=processor.feature_extractor,
        torch_dtype=torch_dtype,
        device=device,
    )
    return pipe

def get_records_transcripts(records: list[str], pipe: Pipeline) -> {}:
    transcripts = {}
    for record in records:
        result = pipe(record, generate_kwargs={"language": "polish"}, return_timestamps=True)
        text = result["text"]
        logging.info(f"transcript: {text}")
        transcripts[record] = text
    return transcripts

def prepare_system_message():
    return Message(
        role="system",
        content="Otrzymasz w kolejnej wiadomości listę transkrypcji zeznań świadków, którzy mogą coś wiedzieć o porwaniu Profesora Andrzeja Maja." +
                   "Na podstawie przesłanych transkrypcji musisz ustalić nazwę na jakiej ulicy znajduje się uczelnia, na której wykłada Andrzej Maj." +
                   "Pamiętaj, że zeznania świadków mogą być sprzeczne, niektórzy z nich mogą się mylić, a inni odpowiadać w dość dziwny sposób." +
                   "Nazwa ulicy nie pada w treści transkrypcji." +
                    "Musisz sam wywnioskować odpowiedź na jakiej ulicy znajduje się uczelnia na której wykłada Profesor Andrzej Maj." 
                    "Jako odpowiedź zwróc tylko nazwę ulicy, bez żadnych dodatkowych informacji.",
    )

def prepare_user_message(transcripts) -> Message:
    content = "Transkcrypcje zeznań świadków:\n"
    for k in transcripts:
        trans = transcripts[k]
        content = content + f"nazwa pliku: {k} | transkrypcja: {trans} . \n\n"
    return Message(
        role="user",
        content=content
    )

def prepare_final_user_message(content: str) -> Message:
    return Message(
        role="user",
        content=f"Z przesłanego stringa musisz WYEKSTRACHOWAć TYLKO nazwę ulicy. String: {content}"
    )

def main():
    dotenv_path = Path('../../.env')
    load_dotenv(dotenv_path=dotenv_path)
    pipe = prepare_pipeline()
    records = get_list_of_records()
    transcripts = get_records_transcripts(records, pipe)

    system_msg = prepare_system_message()
    user_msg = prepare_user_message(transcripts)
    response = call_llm_model([system_msg, user_msg])
    response = call_llm_model([prepare_final_user_message(response.message.content)])

    final_answer = FinalAnswer(
        apikey=os.environ.get("AI_DEVS_API_KEY"),
        task="mp3",
        answer=response.message.content
    )

    call_centrala(final_answer)


def call_centrala(final_answer):
    final_json = json.dumps(final_answer, indent=4, cls=Encoder)
    resp = requests.post(f"{os.environ.get('CENTRALA_HOST')}/report", data=final_json, headers={"Content-Type": "application/json"})
    if resp.status_code != 200:
        logging.error(f"centrala request failed. {resp.status_code} \ {resp.content} ")
        raise Exception
    logging.info(f"centrala request succeeded: {resp.content}")


def call_llm_model(messages: list[Message]) -> Response:
    ollama_request = Request(model=llm_model, stream=False, messages=messages)
    final_json = json.dumps(ollama_request, indent=4, cls=Encoder)
    response = requests.post(f"{os.environ.get('OLLAMA_HOST')}/api/chat", data=final_json, headers={"Content-Type": "application/json"})
    if response.status_code != 200:
        logging.error(f"ollama request failed. {response.status_code} \ {response.content} ")
        raise Exception
    logging.info(f"ollama response:  {response.content}")
    return Response.from_dict(json.loads(response.content))


if __name__ == '__main__':
    main()
