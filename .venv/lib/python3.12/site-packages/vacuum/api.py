#!/usr/bin/env python
#coding:utf-8
# Author        : tuxpy
# Email         : q8886888@qq.com.com
# Last modified : 2016-07-19 14:36:53
# Filename      : api.py
# Description   : 
from __future__ import print_function, unicode_literals

from .config import get_config
import requests
import sys

CONFIG = get_config()

def api_request(method, url, *args, **kwargs):
    url = 'http://' + (CONFIG.get('api_host') or raw_input('please enter api host: ').strip()) + url
    headers = {
            'User-Agent': 'vacuum client',
            'Api-Token': CONFIG.get('api_token') or raw_input('please enter api token: ').strip(),
            }
    response = requests.request(method, url, headers = headers, *args, **kwargs)
    json_data = response.json()
    if json_data['code'] != 0:
        sys.stderr.write(json_data['msg'] + '\n')
        sys.exit(1)

    return response.json()['data']

def fetch_config():
    return api_request('GET', '/api/config')['config']

def _get_receviers(application_name):
    return api_request('GET', '/api/receivers', params = {'application_name': application_name})['receivers']

def get_email_receivers(application_name):
    return _get_receviers(application_name)['email']

def get_sms_receivers(application_name):
    return _get_receviers(application_name)['sms']

