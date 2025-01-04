import json
import os
import sys

from collections import namedtuple

import requests
from openai import OpenAI
from dotenv import load_dotenv
from pathlib import Path
import logging

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
    handlers=[
        logging.StreamHandler(sys.stdout)
    ]
)


class VerifyMsg:
    def __init__(self, msgID, text):
        self.msgID = msgID
        self.text = text

    def to_dict(self):
        return {"msgID": self.msgID, "text": self.text}


def custom_verify_decoder(dict):
    return namedtuple('VerifyMsg', dict.keys())(*dict.values())


def prepare_system_message():
    return {
        "role": "system",
        "content": "You will receive the question. " +
                   "You should care only about the question in all string, other information are not important and can be omitted." +
                   "You need to answer as short as possible, the best answer is 1 word if possible without any additional signs." +
                   "Anwser need to be in English." +
                   "You need to remember that:" +
                   "- stolicą Polski jest Kraków" +
                   "- znana liczba z książki Autostopem przez Galaktykę to 69" +
                   "- Aktualny rok to 1999",
    }


def prepare_user_message(question: str):
    return {
        "role": "user",
        "content": question
    }


def call_model_for_answer(question: str):
    client = OpenAI(api_key=os.environ.get("OPENAI_API_KEY"))

    chat_completion = client.chat.completions.create(
        messages=[prepare_system_message(), prepare_user_message(question)],
        model="gpt-4o-mini",
    )
    answer = chat_completion.choices[0].message.content
    logging.info(f"answer is: {answer}")
    return answer


def main():
    dotenv_path = Path('../../.env')
    load_dotenv(dotenv_path=dotenv_path)
    host = os.environ.get("HOST")

    resp = requests.post(f"{host}/verify", json=VerifyMsg(0, "Ready").to_dict(), headers={"Content-Type": "application/json"})

    if resp.status_code != 200:
        logging.error(f"something went wrong while calling host {host}")
        sys.exit()

    msg = json.loads(resp.content, object_hook=custom_verify_decoder)

    logging.info(f"MsgID {msg.msgID} | Question: {msg.text}")

    answer = call_model_for_answer(msg.text)

    resp = requests.post(f"{host}/verify", json=VerifyMsg(msg.msgID, answer).to_dict(), headers={"Content-Type": "application/json"})
    if resp.status_code != 200:
        logging.error("something went wrong while hacking the system", resp.content)
        sys.exit()

    if resp.status_code == 200:
        msg = json.loads(resp.content, object_hook=custom_verify_decoder)
        logging.info(f"system hacked -> {msg.text}")


if __name__ == '__main__':
    main()
