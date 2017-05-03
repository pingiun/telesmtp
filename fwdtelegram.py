#!/usr/bin/env python3

import sys
import json
import os

from telegram.bot import Bot

TG_TOKEN = '260374523:AAFeVRxVvnRh-ExQNfywEMqZow48vnnh3iw'

GROUP_ID = '-1001104139794'

def main():
	bot = Bot(TG_TOKEN)
	message = json.loads(sys.stdin.read())

	if len(message['to']) > 1:
		to_addresses = ', '.join(message['to'])
	else:
		to_addresses = message['to'][0]
	
	if message['body'] != "":
		bot.send_message(GROUP_ID, "*From*: {}\n*To*: {}\n*Subject*: {}\n\n{}".format(message['from'], to_addresses, message['subject'], message['body']), parse_mode='markdown')
	else:
		bot.send_message(GROUP_ID, "*From*: {}\n*To*: {}\n*Subject*: {}".format(message['from'], to_addresses, message['subject']), parse_mode='markdown')

	with open(os.environ['TELESMTP_MAIL_FILE'], 'rb') as f:
		bot.send_document(GROUP_ID, f)
	print("success")

if __name__ == '__main__':
	main()