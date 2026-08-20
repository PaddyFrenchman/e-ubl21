#!/usr/bin/env python
#coding:utf-8
# Author        : tuxpy
# Email         : q8886888@qq.com.com
# Last modified : 2016-07-14 10:10:18
# Filename      : notifiy.py
# Description   : 
from __future__ import print_function, unicode_literals

from vacuum.config import get_config
from vacuum import history
from third_party.submail.mail_send import MAILSend
from third_party import ucpaas
import time


SUBMAIL_CONFIG = get_config()['submail']
UCPAAS_CONFIG = get_config()['ucpaas']

def get_submail_config():
    return get_config()['submail']

def get_ucpaas_config():
    return get_config()['ucpaas']

def _utf8(s):
    if isinstance(s, unicode):
        return s.encode('utf-8')

    else:
        return s

def send_email(subject, content, to):
    mailer = MAILSend(SUBMAIL_CONFIG)
    [ mailer.add_to(_to) for _to in to]
    mailer.set_sender(SUBMAIL_CONFIG['sender'])
    mailer.set_subject(_utf8(subject))
    mailer.set_text(_utf8(content))
    print(mailer.send())

def send_sms(to, *params):
    latest_send_sms_time = history.get_latest_send_sms_time()
    if latest_send_sms_time and time.time() - latest_send_sms_time < SUBMAIL_CONFIG['send_interval']:
        print('最近刚发过短信')
        return

    for phone in to:
        ucpaas.send_sms(UCPAAS_CONFIG['appid'], UCPAAS_CONFIG['sid'],
                UCPAAS_CONFIG['token'], phone, UCPAAS_CONFIG['template_id'], *params)

    history.update_latest_send_sms_time()

