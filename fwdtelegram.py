#!/usr/bin/env python3

import json
import os
import re
import sys

from telegram.bot import Bot

from database import EmailAddress, TelegramUser


TG_TOKEN = os.environ['TG_TOKEN']
bot = Bot(TG_TOKEN)


def send_message(user, from_, to, subject, body):
    if body == "":
        bot.send_message(user, "*From*: {}\n*To*: {}\n*Subject*: {}".format(from_, to, subject), parse_mode='markdown')
    else:
        bot.send_message(user, "*From*: {}\n*To*: {}\n*Subject*: {}\n\n{}".format(from_, to, subject, body), parse_mode='markdown')
    with open(os.environ['TELESMTP_MAIL_FILE'], 'rb') as f:
        bot.send_document(user, f)

def main():
    message = json.loads(sys.stdin.read())
    address_re = re.compile(r'.*<(.+@.+)>')

    if len(message['to']) > 1:
        to_addresses = ', '.join(message['to'])
    else:
        to_addresses = message['to'][0]

    email = address_re.search(message['delivered_to']).group(1)
    for tg_user in TelegramUser.query.filter(TelegramUser.emailaddresses.any(address=email)):
        print("Sending to", tg_user)
        send_message(str(tg_user.user_id), message['from'], to_addresses, message['subject'], message['body'])

    
    print("success")

if __name__ == '__main__':
    main()