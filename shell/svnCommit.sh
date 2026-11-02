#!/bin/bash
cd $1;
addFile=`svn st | awk '{if ($1 == "?") {print $2} }' | xargs`
if [ "$addFile" != "" ];then 
svn add $addFile
fi
delFile=`svn st | awk '{if ($1 == "!") {print $2} }' | xargs`
if [ "$delFile" != "" ];then 
svn del $delFile
fi
svn ci ./ -m $2
