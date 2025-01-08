import json
import os
import sys

import requests
from openai import OpenAI
from dotenv import load_dotenv
from pathlib import Path
import logging
from json import JSONEncoder
from typing import List, Dict, Any

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
    handlers=[
        logging.StreamHandler(sys.stdout)
    ]
)


class Test:
    def __init__(self, question: str, answer: str):
        self.question = question
        self.answer = answer


    def to_dict(self) -> Dict[str, Any]:
        return {
            "q": self.question,
            "a": self.answer
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Test":
        return cls(
            question=data.get("q"),
            answer=data.get("a")
        )

    def __repr__(self) -> str:
        return f"Test(question={self.question!r}, answer={self.answer!r})"



class TestData:
    def __init__(self, question: str, answer: str, test: Test):
        self.question = question
        self.answer = answer
        self.test = test

    def to_dict(self) -> Dict[str, Any]:
        return {
            "question": self.question,
            "answer": self.answer,
            "test": self.test.to_dict() if self.test else None
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "TestData":
        return cls(
            question=data["question"],
            answer=data["answer"],
            test=Test.from_dict(data.get("test")) if data.get("test") else None
        )

    def __repr__(self) -> str:
        return (
            f"TestData("
            f"question={self.question!r}, "
            f"answer={self.answer!r}, "
            f"test={self.test!r})"
        )



class CalibrationData:
    def __init__(self, apikey: str, description: str, copyright: str, test_data: list[TestData]):
        self.apikey = apikey
        self.description = description
        self.copyright = copyright
        self.test_data = test_data

    def to_dict(self) -> Dict[str, Any]:
        return {
            "apikey": self.apikey,
            "description": self.description,
            "copyright": self.copyright,
            "test-data": [td.to_dict() for td in self.test_data]
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CalibrationData":
        return cls(
            apikey=data["apikey"],
            description=data["description"],
            copyright=data["copyright"],
            test_data=[TestData.from_dict(td) for td in data.get("test-data")]
        )

    def __repr__(self) -> str:
        return (
            f"CalibrationData("
            f"apikey={self.apikey!r}, "
            f"description={self.description!r}, "
            f"copyright={self.copyright!r}, "
            f"test_data={self.test_data!r})"
        )



class FinalAnswer:
    def __init__(self, task: str, apikey: str, answer: CalibrationData):
        self.task = task
        self.apikey = apikey
        self.answer = answer

    def to_dict(self) -> Dict[str, Any]:
        return {
            "task": self.task,
            "apikey": self.apikey,
            "answer": self.answer.to_dict() if self.answer else None
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "FinalAnswer":
        return cls(
            task=data["task"],
            apikey=data["apikey"],
            answer=CalibrationData.from_dict(data.get("answer")) if data.get("answer") else None
        )

    def to_json(self) -> str:
        return json.dumps(self.to_dict())

    @classmethod
    def from_json(cls, json_str: str) -> "FinalAnswer":
        data = json.loads(json_str)
        return cls.from_dict(data)

    def __repr__(self) -> str:
        return (
            f"FinalAnswer("
            f"task={self.task!r}, "
            f"apikey={self.apikey!r}, "
            f"answer={self.answer!r})"
        )


class FinalAnswerEncoder(JSONEncoder):
    def default(self, o):
        if hasattr(o, "to_dict"):
            return o.to_dict()
        return super().default(o)


def prepare_system_message():
    return {
        "role": "system",
        "content": "You will receive the list of questions. " +
                   "You need to answer as short as possible, the best answer is 1 word if possible. " +
                   "The response need to be written in json format. " +
                   "The example of json format is presented below:  " +
                   "[{\"q\":\"What is the capital city of Germany?\",\"a\":\"Berlin\"}]",
    }


def prepare_user_message(questions: list[str]):
    return {
        "role": "user",
        "content": "\n".join(questions)
    }


def call_model_for_answers(questions: list[str]):
    client = OpenAI(api_key=os.environ.get("OPENAI_API_KEY"))

    chat_completion = client.chat.completions.create(
        messages=[prepare_system_message(), prepare_user_message(questions)],
        model="gpt-4o-mini",
    )
    answers = chat_completion.choices[0].message.content
    logging.info(f"answers are: {answers}")
    return answers


def validate_calculation(question: str) -> int:
    split = question.split(" + ")
    first = split[0]
    second = split[1]
    return int(first) + int(second)


def main():
    dotenv_path = Path('../../.env')
    load_dotenv(dotenv_path=dotenv_path)

    with open('json.txt', 'r') as file:
        file_contents = file.read()
        calibration_data = CalibrationData.from_dict(json.loads(file_contents))

    questions_to_model = []
    for data in calibration_data.test_data:
        if data.test is not None and data.test.question is not None and len(data.test.question) > 0:
            questions_to_model.append(data.test.question)

    if len(questions_to_model) == 0:
        logging.error("questions not found")
        sys.exit(1)


    logging.info("there are questions to the model")
    answers = call_model_for_answers(questions=questions_to_model)
    parsed_answers = [Test.from_dict(item) for item in json.loads(answers)]

    for test_data in calibration_data.test_data:
        for t in parsed_answers:
            if test_data.test is not None and test_data.test.question == t.question:
                test_data.test.answer = t.answer
                break

        test_data.answer = validate_calculation(test_data.question)

    final_answer = FinalAnswer(
        apikey=os.environ.get("AI_DEVS_API_KEY"),
        task="JSON",
        answer=calibration_data
    )

    final_json = json.dumps(final_answer, indent=4, cls=FinalAnswerEncoder)
    resp = requests.post(f"{os.environ.get('CENTRALA_HOST')}/report", data=final_json, headers={"Content-Type": "application/json"})

    if resp.status_code == 400:
        logging.error(f"bad request: {resp.content}")
    elif resp.status_code == 200:
        logging.info(f"final report sent with success")
        logging.info(resp.content)


if __name__ == '__main__':
    main()
