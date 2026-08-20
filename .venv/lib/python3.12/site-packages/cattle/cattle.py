#!/usr/bin/env python
#coding:utf8
# Author          : tuxpy
# Email           : q8886888@qq.com
# Last modified   : 2015-03-19 16:35:52
# Filename        : cattle/cattle.py
# Description     : API地址：http://docs.qiniutek.com/v3/api/io/#upload

from __future__ import unicode_literals
__all__ = ['Cattle']

from .token import UploadToken, AccessToken, DownloadToken
from os import path

RS_HOST = 'http://rs.qiniu.com'
RSF_HOST = 'http://rsf.qbox.me'
UP_HOST = 'http://upload.qiniu.com'
UC_HOST = 'http://uc.qbox.me'

from .request import post, get
from .utils import urlsafe_b64encode, native_str
from mimetypes import guess_type
from time import time
from functools import partial
import hashlib

def get_file_mime_type(file_path):
    return guess_type(file_path)[0] or ''

class Cattle():
    def __init__(self, access_key, secret_key):
        self.access_key = access_key
        self.secret_key = secret_key
        self.upload_tokens = {}
        self.rm = partial(self.__rs_handler, 'delete')
        self.stat = partial(self.__rs_handler, 'stat')
        self.cp = partial(self.__rs_handler_two, 'copy')
        self.mv = partial(self.__rs_handler_two, 'move')
    
    def __put(self, scope, key, content, mime_type='', override=True, prefix = None):
        scope, key = native_str(scope, key)
        if prefix:
            key = native_str(prefix) + key
        if override:
            token = self.get_upload_token('%s:%s' %(scope, key))
        else:
            token = self.get_upload_token(scope)

        data = {
                'auth': token,
                'action': '/rs-put/%s/mimeType/%s' %  
                (urlsafe_b64encode('%s:%s' % (scope, key)),
                    urlsafe_b64encode(mime_type))
                }
        files = {'file': content}

        url = UP_HOST + '/upload'

        ret = post(url, files = files, data = data)

        # 如果失败了的话，这个ret里面包含的是错误信息
        if ret.status_code == 200:
            return ret.json(), ''
        else:
            return '', ret.json()

    def put_data(self, scope, data, key = None, mime_type = '', override = True, prefix = None):
        """
        上传二进制流
        """
        if not key:
            key = hashlib.md5(data).hexdigest()

        return self.__put(scope, key, data, mime_type, override, prefix)

    def put_file(self, scope, file_path, key = None, mime_type = '', override = True, md5 = False, prefix = None):
        """
        直传文件
        如果不指定key，同时md5为False，则会以 file_path作为 key，如果指定了md5对内容做一个md5计算，以md5值作为key
        """
        with open(file_path, 'rb') as fd:
            content = fd.read()

        key = key or (md5 and hashlib.md5(content).hexdigest() + path.splitext(file_path)[1]) or file_path

        return self.__put(scope, key, content, 
                mime_type or get_file_mime_type(file_path),  override, prefix)

    def __rs_handler(self, do, scope, key):
        scope, key = native_str(scope, key)
        url = '%s/%s/%s' % (RS_HOST, do,
                urlsafe_b64encode('%s:%s' % (scope, key)))
        return self.api_call(url)


    def __rs_handler_two(self, do, scope_s, key_s, scope_d, key_d = None):
        scope_s, key_s, scope_d, keys_d = native_str(scope_s, key_s, scope_d, key_d)
        # 如果不指定scope_d，则表示在同一个bucket中进行操作
        if not key_d:
            scope_d, key_d  = scope_s, scope_d

        encoded_src = urlsafe_b64encode('%s:%s' %(scope_s, key_s))
        encoded_dest = urlsafe_b64encode('%s:%s' %(scope_d, key_d))
        url = '%s/%s/%s/%s' % (RS_HOST, do,
                encoded_src, encoded_dest)
        return self.api_call(url)

    def ls(self, scope, marker = '', limit = 1000, prefix=''):
        """
        列出文件
        limit: 限制列出的数量
        prefix: 限制列出文件的前缀
        """
        url = RSF_HOST + '/list?bucket={scope}&marker={marker}&limit={limit}&prefix={prefix}'.format(
               scope = scope, marker = marker, 
                limit = limit, prefix = prefix)

        return self.api_call(url)

    def drop(self, scope):
        url = RS_HOST + '/drop/{scope}'.format(scope = scope)

        return self.api_call(url)

    def empty(self, scope):
        for obj in self.ls_all(scope):
            key = obj['key']
            self.rm(scope, key)

    def ls_all(self, scope, prefix = ''):
        """
        列出所有文件
        prefix: 限制列出文件的前缀
        """
        _list = []
        marker = ''
        while True:
            ret, error = self.ls(scope, marker = marker, prefix = prefix)
            assert not error
            _list.extend(ret['items'])
            marker = ret.pop('marker', '')
            if not marker:
                break

        return _list

    def private_url(self, url, ttl=3600):
        """
        url格式:http://域名/key
        ttl: 下载链接过期时间
        """
        url += '?attname=&e=%s' % (int(time()) + ttl)
        token = self.get_download_token(url)
        return url + '&token=' +  token

    def api_call(self, url, data = {}):
        rs_headers = self.get_rs_headers(url)
        ret = post(url, headers = rs_headers, data = data)
        if ret.status_code == 200:
            return ret.text.strip() and ret.json(), ''
        else:
            return '', ret.json()

    def get_upload_token(self, scope, ttl=3600):
        return self.upload_tokens.setdefault(scope, 
                UploadToken(self.access_key, self.secret_key, scope)).token

    def get_access_token(self, url):
        return AccessToken(self.access_key, 
                self.secret_key, url = url).token

    def get_download_token(self, url):
        return DownloadToken(self.access_key, self.secret_key, url).token

    def get_rs_headers(self, url):
        token = self.get_access_token(url)
        return {
                'Content-Type': ' application/x-www-form-urlencoded',
                'Authorization': 'QBox %s' % token,
                }

    def get_bucket(self, scope):
        return Bucket(scope, self)


    def list_bucket(self):
        url = '%s/buckets' % (RS_HOST)
        return self.api_call(url)


class Bucket():
    def __init__(self, scope, _cattle):
        self.put_file = partial(_cattle.put_file, scope)
        self.put_data = partial(_cattle.put_data, scope)
        self.rm = partial(_cattle.rm, scope)
        self.ls = partial(_cattle.ls, scope)
        self.stat = partial(_cattle.stat, scope)
        self.ls_all = partial(_cattle.ls_all, scope)
        self.cp = partial(_cattle.cp, scope)
        self.mv = partial(_cattle.mv, scope)
        self.private_url = _cattle.private_url
        self.drop = partial(_cattle.drop, scope)
        self.empty = partial(_cattle.empty, scope)

