import os

from sqlalchemy import Column, BigInteger, String, Table, Integer, ForeignKey
from sqlalchemy import create_engine
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import scoped_session, sessionmaker, relationship


engine = create_engine(os.environ['DB_URL'], echo=True)

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