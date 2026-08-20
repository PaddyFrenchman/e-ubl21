#!/usr/bin/env python
#coding:utf-8
# Author        : tuxpy
# Email         : q8886888@qq.com.com
# Last modified : 2016-01-29 15:27:35
# Filename      : ucpaas.py
# Description   : 
from __future__ import print_function

import requests
import base64
import time
import json
import hashlib

__ALL__ = ['send_sms']

def made_authorization(sid, t):
    return base64.encodestring(sid + ':' + t).strip()

def made_sig(sid, token, t):
    return hashlib.md5(sid + token + t).hexdigest().upper()


def _send_sms(appid, sid, token, phone, template_id, *params):
    t = time.strftime('%Y%m%d%H%M%S')
    params_str = ','.join([ str(param) for param in params])
    headers = {
            'Accept'            :       'application/json',
            'Content-Type'      :       'application/json;charset=utf-8',
            'Authorization'     :       made_authorization(sid, t),
            }

    json_data =  '{"templateSMS":{ "appId":"%s","to":"%s","templateId":"%s","param":"%s"}}' % (appid,  phone, template_id, params_str) 

    api_url =  'https://api.ucpaas.com/{app_version}/Accounts/{sid}/Messages/templateSMS?sig={sig}'.format(
            app_version = '2014-06-30', sid = sid, sig = made_sig(sid, token, t))
    response = requests.request(**{
        'method'            :       'POST',
        'data'              :       json_data,
        'url'               :       api_url,
        'headers'           :       headers,
        })

    success = response.json()['resp']['respCode'] == '000000'

    if not success:
        print(response.content)

    return success


def send_sms(*args, **kwargs):
    try:
        return _send_sms(*args, **kwargs)
    except Exception as ex:
        return False

