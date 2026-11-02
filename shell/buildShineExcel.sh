#!/bin/sh
#更新策划表格
cd $1
svn up .

#用最近版本判断表格目录是否有更新
lastVersion=`svn info|grep "最后修改的版本"|awk '{print $2}'`
cd data
lastDataVersion=`svn info|grep "最后修改的版本"|awk '{print $2}'`
if [ "${lastVersion}" -gt "${lastDataVersion}" ]
then
echo "表格已是最新，无需构建！"
exit
fi

cd "$3"
projectPath=$1/excelTools
./Unity.app/Contents/MacOS/Unity -quit -batchmode -projectPath $projectPath -executeMethod AutoBuild.StartBuild -buildMethod:BuildDataConfig -webHook:$4 -otherParams:$5
