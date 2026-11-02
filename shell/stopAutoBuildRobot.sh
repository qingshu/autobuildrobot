#!/bin/sh
processName="autobuildrobot_mac_darwin"
pid=`ps aux|grep $processName|grep -v grep|awk '{print $2}'`
if [ -n "$pid" ];then
    kill -9 $pid
    echo "pid=$pid,kill $processName successfull!"
else
    echo "No process exist of $processName $pid"
fi
processName="wyc_mac_64"
pid=`ps aux|grep $processName|grep -v grep|awk '{print $2}'`
if [ -n "$pid" ];then
    kill -9 $pid
    echo "pid=$pid,kill $processName successfull!"
else
    echo "No process exist of $processName $pid"
fi
