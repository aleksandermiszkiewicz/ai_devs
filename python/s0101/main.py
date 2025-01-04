import os
import sys

import requests
from openai import OpenAI
from dotenv import load_dotenv
from pathlib import Path
import logging
from bs4 import BeautifulSoup as BS

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
    handlers=[
        logging.StreamHandler(sys.stdout)
    ]
)


def prepare_system_message():
    return {
        "role": "system",
        "content": "You will receive the question. You need to answer as short as possible, the best answer is 1 word if possible."
    }

def prepare_user_message(question: str):
    return {
        "role": "user",
        "content": question
    }

def extract_question(html: str):
    soup = BS(html, "html.parser")
    p_tag = soup.find('p', id="human-question")

    if p_tag:
        question = p_tag.get_text(separator=' ').replace('Question:', '').strip()
        logging.info(f"question is: {question}")
        return question
    return None


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
    resp = requests.get(host)

    if resp.status_code != 200:
        logging.error(f"something went wrong while calling host {host}")
        sys.exit()

    question = extract_question(resp.content)
    if question is None:
        logging.error("failed to extract question")
        sys.exit()

    answer = call_model_for_answer(question)

    payload = {'username':os.environ.get("AGENT_USER"),'password':os.environ.get("AGENT_USER"), 'answer': answer}
    response = requests.post(host, files=payload)
    if response.status_code != 200:
        logging.error("something went wrong while logging to the system", response.content)
        sys.exit()

    if response.status_code == 200:
        logging.info("logging with success")

if __name__ == '__main__':
    main()