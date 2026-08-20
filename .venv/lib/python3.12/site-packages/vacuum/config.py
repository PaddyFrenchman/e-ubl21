#!/usr/bin/env python
#coding:utf-8
# Author        : tuxpy
# Email         : q8886888@qq.com.com
# Last modified : 2016-07-12 15:43:06
# Filename      : config.py
# Description   : 
from __future__ import print_function, unicode_literals

from vacuum import datatype
import os
import sys
import json

RC_FILE = os.path.join(os.path.expanduser('~'), '.vacuumrc')

def _get_config_from_rc():
    if not os.path.exists(RC_FILE):
        return {}

    with open(RC_FILE, 'rb') as fd:
        content = fd.read()
        return json.loads(content)

def _get_log_path():
    return os.environ.get('DSB_WARNING_LOG', '/var/log/vacuum')

def save_config(config):
    with open(RC_FILE, 'wb') as fd:
        fd.write(json.dumps(config, indent = 4))

def get_config():
    config_from_rc = _get_config_from_rc()
    if not config_from_rc:
        return {}

    config = datatype.ObjectDict(config_from_rc)
    config['log_path'] = _get_log_path()

    return config

