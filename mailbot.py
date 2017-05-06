import logging
import os

from functools import wraps

from telegram import InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import CommandHandler, CallbackQueryHandler, MessageHandler, ConversationHandler, Updater

from database import EmailAddress, TelegramUser, db_session


logging.basicConfig(level=logging.DEBUG)

MASTER = '167921906'


def request_start(bot, update, args):
    if len(args) == 1:
        update.message.text = args[0]
        return request(bot, update)
    bot.send_message(chat_id=update.message.chat_id,
        text="Please send an email address for which you want to receive messages.")
    return 'enter_email'

def request(bot, update):
    if not '@' in update.message.text:
        bot.send_message(chat_id=update.message.chat_id, 
            text="Your message doesn't look like an email address... You can try again though.")
        return 'enter_email'
    
    name = update.message.from_user.first_name
    if update.message.from_user.last_name != "":
        name += " " + update.message.from_user.last_name
    if update.message.from_user.username != "":
        name += " (" + update.message.from_user.username + ")"

    keyboard = [[InlineKeyboardButton("Yep!", callback_data='{}:{}'.format(update.message.text.lower(), update.message.chat_id))], [InlineKeyboardButton("Nope", callback_data='nope')]]
    bot.send_message(chat_id=MASTER, text="Approve this email address?\nEmail: {}\nUser: {}".format(update.message.text.lower(), name), reply_markup=InlineKeyboardMarkup(keyboard))

    bot.send_message(chat_id=update.message.chat_id,
        text="Alright I sent a message to my master to check this.")
    return ConversationHandler.END

def add_link(user_id, address):
    user = TelegramUser.query.filter(TelegramUser.user_id == user_id).first()
    email = EmailAddress.query.filter(EmailAddress.address == address).first()
    if email is None:
        email = EmailAddress(address=address)
    if user is None:
        user = TelegramUser(user_id=user_id)
    user.emailaddresses.append(email)

    db_session.add(user)
    db_session.commit()


def button(bot, update):
    query = update.callback_query

    if query.data == 'nope':
        bot.editMessageText(text="Request denied",
                        chat_id=query.message.chat_id,
                        message_id=query.message.message_id)
    else:
        split = query.data.split(':')
        add_link(split[1], split[0])
        bot.editMessageText(text="Request for {} accepted".format(split[0]),
                        chat_id=query.message.chat_id,
                        message_id=query.message.message_id)
        bot.send_message(chat_id=split[1], text="Your request for {} has been accepted".format(split[0]))

def cancel(bot, update):
    bot.send_message(chat_id=update.message.chat_id, text="Canceled current command.")
    return ConversationHandler.END

def main():
    updater = Updater(token=os.environ['TG_TOKEN'])
    dispatcher = updater.dispatcher

    cancel_command = CommandHandler('cancel', cancel)
    updater.dispatcher.add_handler(CallbackQueryHandler(button))

    request_command = CommandHandler('request', request_start, pass_args=True)
    request_handler = MessageHandler(None, request)
    dispatcher.add_handler(ConversationHandler(
        [request_command], {'enter_email': [request_handler]}, [cancel_command]))

    updater.start_polling()

if __name__ == '__main__':
    main()