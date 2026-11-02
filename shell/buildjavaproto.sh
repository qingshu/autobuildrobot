#!/bin/sh
#先更新
curDir=`cd $(dirname $0); pwd -P`
tarDir=$1
cd $tarDir
svn up .

#用最近版本判断是否有更新
lastVersion=`svn info|grep "最后修改的版本"|awk '{print $2}'`
cd proto
lastProtoVersion=`svn info|grep "最后修改的版本"|awk '{print $2}'`
if [ "${lastVersion}" -gt "${lastProtoVersion}" ]
then
echo "消息码已是最新，无需构建！"
exit
fi
cd ..

#原始文件和生成文件路径
proto_pth=${tarDir}/proto
java_out_path=${tarDir}/java
lua_out_path=${tarDir}/lua

#生成pb文件
cd $curDir/proto
find ${proto_pth} -type f -name "*.proto" -exec protoc -I=${proto_pth} --java_out=$java_out_path {}\;
find ${proto_pth} -type f -name "*.proto" -exec protoc -I=${proto_pth} --lua_out=$lua_out_path --plugin=protoc-gen-lua=./protoc-gen-lua/plugin/protoc-gen-lua.sh {}\;


