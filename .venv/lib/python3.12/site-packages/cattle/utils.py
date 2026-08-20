#!/usr/bin/env python
#coding:utf8
# Author          : tuxpy
# Email           : q8886888@qq.com
# Last modified   : 2015-03-19 16:40:12
# Filename        : cattle/utils.py
# Description     : 

from __future__ import unicode_literals
import base64
import sys

is_py3 = sys.version[0] == '3'

def urlsafe_b64encode(s):
    """
    为了兼容python2的，返回str，并不返回 bytes
    """
    if not isinstance(s, bytes):
        s = s.encode('utf-8')
    value = base64.urlsafe_b64encode(s)

    return value.decode()

def native_str(*args):
    """
    把py2 的参数中str转成unicode
    """
    if is_py3:
        result = args
    else:
        result = map(lambda s: isinstance(s, str) and  s.decode('utf-8') or s, args)
    
    return result if len(args) != 1 else result[0]

def utf8(s):
    if is_py3:
        return isinstance(s, str) and s.encode('utf-8') or s
    else:
        return isinstance(s, unicode) and s.encode('utf-8') or s

