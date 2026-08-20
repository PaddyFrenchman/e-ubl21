#!/usr/bin/env python
#coding:utf-8
# Author        : tuxpy
# Email         : q8886888@qq.com.com
# Last modified : 2016-07-12 16:47:33
# Filename      : log.py
# Description   : 

from __future__ import print_function, unicode_literals
from cattle import Cattle
from vacuum import api
from vacuum import history 
from vacuum import notifiy
from vacuum.config import get_config
import datetime
import hashlib
import json
import os
import re
import time
import zipfile

CONFIG = get_config()

def compile_warning_rules(warning_rules):
    for _rule in warning_rules:
        name = _rule['name']
        if name[0] != '^':
            name = '^.*' + name

        if name[-1] != '$':
            name = name + '.*$'

        _rule['name_re'] = re.compile(r'%s' % (name, ))

    return warning_rules

compile_warning_rules(CONFIG['warning_rules'])

class Dust(object):
    def __init__(self, record):
        self.record = record.strip()
        self.data = json.loads(record)
        for name, value in self.data.items():
            setattr(self, name, value)

        self.warning_rule = self.get_warning_rule()
        self.data['warning_rule_name'] = self.warning_rule_name = self.warning_rule['name']

    def out(self):
        return json.dumps(self.data)

    def __repr__(self):
        return "< Dust warning_rule: {warning_rule_name}, exception_name: {exception_name}, timestamp: {timestamp}>".format(
                 **self.data)

    def get_id(self, *params):
        """通过日志的参数来生成该条日志的标记, 标记不唯一。标记相同的日志表示是同一条日志。如果不传，则采用exception_name, func_name来确定标记"""
        params = sorted(params or ['exception_name', 'func_name'])
        md5_instance = hashlib.md5()
        for param in params:
            md5_instance.update(param + str(self.data[param]))

        return md5_instance.hexdigest()

    def get_warning_rule(self):
        exception_name = self.exception_name.lower().strip()
        default_rule = None
        
        for _rule in CONFIG.warning_rules:
            if _rule['name'] == 'default': 
                default_rule = _rule

            if _rule['name_re'].match(exception_name):
                return _rule

        assert default_rule, 'miss default warning rule'

        return default_rule

    def ok(self, condition):
        """第一版本只判断and关系"""
        if not condition:
            return True

        condition = self.unwind_condition(condition)
        for name, value, opera in condition:
            opera_method = getattr(self, 'is_' + opera)
            if not opera_method(name, value):
                return False

        return True

    def unwind_condition(self, condition):
        result = []
        for name, value_expression in condition.items():
            if isinstance(value_expression, dict) and all([key[0] == '$' for key in value_expression.keys()]):
                for opera, value in value_expression.items():
                    opera = opera[1:]
                    assert hasattr(self, 'is_' + opera), 'opera not defined'

                    result.append((name, value, opera))

            else:
                value = value_expression
                result.append((name, value, 'eq'))

        return result

    def __getitem__(self, name):
        return getattr(self, name)

    def is_eq(self, name, value):
        return self[name] == value

    def is_neq(self, name, value):
        return self[name] != value

    def is_gte(self, name, value):
        return self[name] >= value

    def is_gt(self, name, value):
        return self[name] > value

    def is_lt(self, name, value):
        return self[name] < value

    def is_lte(self, name, value):
        return self[name] < value

class Logger(object):
    def __init__(self, _file):
        assert os.path.isfile(_file), 'not found logger file: %s' % (_file, )
        self._file = _file

    def _iter_line(self):
        for line in self.fd:
            yield line

    def init_iter(self):
        if hasattr(self, 'fd'):
            self.fd.close()

        self.fd = open(self._file, 'rb')
        self._iter = self._iter_line()

    def find(self, condition = None):
        self.init_iter()

        meet_dusts = []
        for dust in self:
            if dust.ok(condition):
                meet_dusts.append(dust)

        return meet_dusts


    def __iter__(self):
        return self

    def __exit__(self, *args, **kwargs):
        self.fd.close()

    def next(self):
        return Dust(self._iter.next())

    def size(self):
        return os.stat(self._file).st_size

class DustGroup(object):
    def __init__(self, dusts):
        self.dusts = dusts

    def ls(self):
        return self.dusts

    def out(self):
        return '\n'.join([ dust.out() for dust in self.dusts])

    def by(self, *params):
        """通过指定params来对dust做分组, 返回
        [
        [dust, ...],
        [dust, ...],
        ]
        """
        dust_map = {}
        for dust in self.dusts:
            dust_map.setdefault(dust.get_id(*params), []).append(dust)

        return dust_map.values()

def ls(what_date = None):
    """
    列出所有的日志, 返回[(application1 name, exception log), ]
    """
    logs = []
    what_date = what_date or time.strftime("%Y/%m/%d")
    for application in os.listdir(CONFIG.log_path):
        exception_log = os.path.join(CONFIG.log_path, application, what_date + "_exception.log")
        if not os.path.exists(exception_log):
            continue
        logs.append((application, exception_log))

    return logs

def report_analysis_summay(application_name, dusts):
    warning_rule = dusts[0].warning_rule
    subject = '{application_name} 出异常了，规则名: {rule_name}, 每{intervals}秒只能出现{threshold}次异常, 实际出现了{actual_dust_numbers}'.format(
        rule_name = warning_rule['name'],
        intervals = warning_rule['intervals'],
        threshold = warning_rule['threshold'],
        application_name = application_name,
        actual_dust_numbers = len(dusts),
        )
    print(subject)
    content = DustGroup(dusts).out()
    print(content)

    notifiy.send_email(subject, content, api.get_email_receivers(application_name))
    notifiy.send_sms(api.get_sms_receivers(application_name), 'exception')

def analysis_same_dust(application_name, dusts):
    if not dusts:
        return
    warning_rule = dusts[0].warning_rule
    exception_begin_timestamp = dusts[0].timestamp
    need_warning_dusts = []
    max_warning_dusts = []

    latest_exception_time = history.get_latest_exception_time(application_name, warning_rule['name'])
    
    for dust in dusts:
        if latest_exception_time and dust.timestamp < latest_exception_time: # 如果当前异常发生在上一次找到的有异常的异常日志前，则表示之前已经预警过了，不再计算
            continue

        if dust.timestamp >= exception_begin_timestamp + warning_rule['intervals']: # 如果超过了某一个时间间隔，则需要重新记数
            exception_begin_timestamp = dust.timestamp
            over_threshold = len(need_warning_dusts) > warning_rule['threshold']
            over_threshold and history.update_latest_exception_time(application_name, warning_rule['name'], dust.timestamp)

            if over_threshold and len(need_warning_dusts) >  len(max_warning_dusts): # 把异常数据最多的那个时间段记录下来
                max_warning_dusts = need_warning_dusts

            need_warning_dusts = []

        need_warning_dusts.append(dust)

    need_warning_dusts = max_warning_dusts or need_warning_dusts
    if len(need_warning_dusts) > warning_rule['threshold']:
        history.update_latest_exception_time(application_name, warning_rule['name'], dusts[-1].timestamp)

    else:
        return

    return need_warning_dusts

def choice_best_timestamp_range(application_name):
    """根据历史日志扫描结果来选择一个最佳的时间范围.找出上次扫描中异常时间最早的时间作为该次扫描的日志起启时间"""
    all_latest_exception_time  = [ history.get_latest_exception_time(application_name, _rule['name']) for _rule in CONFIG.warning_rules]
    all_latest_exception_time = [ _time for _time in all_latest_exception_time if _time]
    if not all_latest_exception_time:
        return time.time() - 30 * 60 # 默认时间是前30分钟

    min_latest_exception_time = min([ _time for _time in all_latest_exception_time if _time])
    return max(min_latest_exception_time, time.time() - 30 * 60)

def analysis(application_name, log_file):
    logger = Logger(log_file)
    dusts = logger.find({
        'timestamp': {"$gte": choice_best_timestamp_range(application_name)}, # 取出最近30分钟内的数据
        })

    dust_group = DustGroup(dusts)
    dusts_groups = dust_group.by('warning_rule_name')
    for dusts in dusts_groups:
        exception_dusts = analysis_same_dust(application_name, dusts)
        if not exception_dusts:
            continue

        report_analysis_summay(application_name, exception_dusts)

### qiniu ######
def get_bucket():
    return Cattle(CONFIG.qiniu['ak'], CONFIG.qiniu['sk']).get_bucket(CONFIG.qiniu['bucket'])

def upload(file_path, cdn_path):
    bucket = get_bucket()

    log_zip_path = file_path + '.zip'
    log_zip = zipfile.ZipFile(log_zip_path, 'w', compression = zipfile.ZIP_DEFLATED)
    log_zip.write(file_path, arcname = file_path[len(CONFIG.log_path) + 1:])
    log_zip.close()

    bucket.put_file(log_zip_path, key = os.path.basename(log_zip_path), prefix = cdn_path)

def sync2cdn():
    yesterday = (datetime.date.today() - datetime.timedelta(1)).strftime("%Y/%m/%d")
    for application_name, log_file in ls(yesterday):
        cdn_path = os.path.dirname(log_file)[len(CONFIG.log_path) + 1: ] + '/'
        upload(log_file, cdn_path)

