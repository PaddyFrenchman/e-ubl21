#!/usr/bin/env python
#coding:utf-8
# Author        : tuxpy
# Email         : q8886888@qq.com.com
# Last modified : 2016-07-12 15:47:12
# Filename      : datetype.py
# Description   : 
from __future__ import print_function, unicode_literals


class ObjectDict(dict):
    def __getattr__(self, name):
        return self[name]

    def __setattr__(self, name, value):
        self[name] = value

