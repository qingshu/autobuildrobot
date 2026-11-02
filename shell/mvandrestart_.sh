#!/bin/bash
svrRoot=$1
dirname=$2
zipfile=${dirname}".zip"
backfile=${dirname}".tgz"
excuteFile="startAll.sh"

#如果文件夹不存在，则创建文件夹
cd ${svrRoot}
if [ ! -d "$dirname" ]; then
mkdir $dirname
fi

#备份
tar -czvf ${backfile} --exclude="*/logdata" --exclude=${zipfile} --exclude="*/.svn" ${dirname}

#先关服再解压
cd ${dirname}
if [ ! -x "$excuteFile" ]; then
if [ -e "stop.sh" ];then
#如果存在脚本才执行（有些是没有可执行文件的）
./stop.sh;sleep 5;
fi
else
./stopAll.sh;sleep 10;
fi

#解压
cd ..
unzip -o ${zipfile} -d ${dirname}

#后台是拷贝文件夹下压缩包，所以这里拷贝一下
sleep 2
mv -f ${zipfile} ${dirname}

#启动
cd ${dirname}
if [ ! -x "$excuteFile" ]; then
if [ -e "start.sh" ];then
#如果存在脚本才执行（有些是没有可执行文件的）
./start.sh;exit
fi
else
./startAll.sh;exit
fi
