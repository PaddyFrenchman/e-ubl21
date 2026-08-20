#!/usr/bin/env python
#coding:utf-8
# Author        : tuxpy
# Email         : q8886888@qq.com.com
# Last modified : 2015-06-20 12:40:03
# Filename      : t.py
# Description   : 
from request import Request

r = Request(method = 'POST', url = 'http://127.0.0.1:8080/', data = {'name': 'ljd'}, files={'file': 'sdfsd'})
response = r.post()

