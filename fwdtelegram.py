#!/usr/bin/env python3

import json
import os
import re
import sys

from telegram.bot import Bot

from sqlalchemy import Column, BigInteger, String, Table, Integer, ForeignKey
from sqlalchemy import create_engine
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import scoped_session, sessionmaker, relationship

TG_TOKEN = '260374523:AAFeVRxVvnRh-ExQNfywEMqZow48vnnh3iw'
bot = Bot(TG_TOKEN)

engine = create_engine(os.environ["DB_URL"], echo=True)

db_session = scoped_session(sessionmaker(autocommit=False,
                                         autoflush=False,
                                         bind=engine))

Base = declarative_base()
Base.query = db_session.query_property()

association_table = Table('email_telegram', Base.metadata,
    Column('emailaddresses_id', Integer, ForeignKey('emailaddresses.id')),
    Column('telegramusernames_id', Integer, ForeignKey('telegramusernames.id'))
)

class EmailAddress(Base):
    __tablename__ = 'emailaddresses'
    id = Column(Integer, primary_key=True)
    address = Column(String)
    telegramusernames = relationship(
        "TelegramUser",
        secondary=association_table,
        back_populates="emailaddresses")

class TelegramUser(Base):
    __tablename__ = 'telegramusernames'
    id = Column(Integer, primary_key=True)
    user_id = Column(BigInteger)
    emailaddresses = relationship(
        "EmailAddress",
        secondary=association_table,
        back_populates="telegramusernames")

Base.metadata.create_all(bind=engine)

def send_message(user, from_, to, subject, body):
    if body == "":
        bot.send_message(user, "*From*: {}\n*To*: {}\n*Subject*: {}".format(from_, to, subject), parse_mode='markdown')
    else:
        bot.send_message(user, "*From*: {}\n*To*: {}\n*Subject*: {}".format(from_, to, subject, body), parse_mode='markdown')
    with open(os.environ['TELESMTP_MAIL_FILE'], 'rb') as f:
        bot.send_document(user, f)

def main():
    message = json.loads(sys.stdin.read())
    address_re = re.compile(r'.*<(.+@.+)>')

    if len(message['to']) > 1:
        to_addresses = ', '.join(message['to'])
    else:
        to_addresses = message['to'][0]

    for address in message['to']:
        email = address_re.search(address)[1]
        for tg_user in TelegramUser.query.filter(TelegramUser.emailaddresses.any(address=email)):
            send_message(str(tg_user.user_id), message['from'], message['to'], message['subject'], message['body'])

    
    print("success")

if __name__ == '__main__':
    main()