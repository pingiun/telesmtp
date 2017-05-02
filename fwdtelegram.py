#!/usr/bin/env python3

import sys
import json
import os

from telegram.bot import Bot

TG_TOKEN = ''

GROUP_ID = '-1001104139794'

def main():
	bot = Bot(TG_TOKEN)
	message = json.loads(sys.stdin.read())
	bot.send_message(GROUP_ID, "New message from: " + message['from'] + "\nSubject: " + message['subject'])
	with open(os.environ['TELESMTP_MAIL_FILE'], 'rb') as f:
		bot.send_document(GROUP_ID, f)
	print("success")

if __name__ == '__main__':
	main()