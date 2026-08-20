#!/usr/bin/env python
#coding:utf-8
# Author        : tuxpy
# Email         : q8886888@qq.com.com
# Last modified : 2016-07-13 16:36:40
# Filename      : history.py
# Description   : 
from __future__ import print_function, unicode_literals
import multiprocessing
import os
import json
import time

LOCK = multiprocessing.RLock()

HISTORY_FILE = os.path.join(os.path.expanduser('~'), '.vacuum_history')

def get():
    if not os.path.exists(HISTORY_FILE):
        return {}

    with open(HISTORY_FILE, 'rb') as fd:
        return json.loads(fd.read())

def save(history):
    with LOCK:
        with open(HISTORY_FILE, 'wb') as fd:
            fd.write(json.dumps(history))

def update_latest_exception_time(application_name, warning_rule_name, timestamp):
    old_history = get()
    key = '%s:%s:%s' % (application_name, warning_rule_name, 'latest_exception_time')
    old_history[key] = timestamp
    save(old_history)

def get_latest_exception_time(application_name, warning_rule_name):
    key = '%s:%s:%s' % (application_name, warning_rule_name, 'latest_exception_time')
    return get().get(key)

def update_latest_send_sms_time():
    old_history = get()
    old_history['latest_send_sms_time'] = time.time()
    save(old_history)

def get_latest_send_sms_time():
    return get().get('latest_send_sms_time')

