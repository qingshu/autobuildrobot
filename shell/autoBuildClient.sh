#!/bin/sh
set -eu
[ "$#" -eq 8 ] || { echo "客户端构建参数不足" >&2; exit 2; }
enginePath=$1
projectPath=$2
cd "$projectPath" || exit 1
if [ -n "$7" ]; then
    echo "多平台构建，当前为 $7"
    [ -d "Library_$7" ] || { echo "目标平台 Library_$7 不存在" >&2; exit 1; }
    if [ -L Library ]; then
        rm Library
    elif [ -e Library ]; then
        echo "Library 不是符号链接，请先备份并配置平台缓存" >&2
        exit 1
    fi
    ln -s "Library_$7" Library
fi
status=$(svn status) || exit 1
printf '%s\n' "$status" | while IFS= read -r line; do
    case "$line" in
        \?*)
            file=$(printf '%s\n' "$line" | sed 's/^? *//')
            case "$file" in *Library*|*GameRes*|*Backup*|*xcode*|*StreamingAssets*|*Lua*) continue ;; esac
            case "$file" in ''|/*|../*|*/../*|..|.|[A-Za-z]:*) echo "拒绝清理异常路径: $file" >&2; exit 1 ;; esac
            rm -rf -- "$file"
            ;;
    esac
done
status=$(svn status --ignore-externals) || exit 1
printf '%s\n' "$status" | while IFS= read -r line; do
    case "$line" in
        'M '*|'C '*|'! '*|'~ '*)
            file=$(printf '%s\n' "$line" | cut -c9-)
            case "$file" in *.meta|*.png|*.PNG|*.jpg) continue ;; esac
            [ -z "$file" ] || svn revert --depth=infinity -- "$file"
            ;;
    esac
done
svn update . --accept tf
status=$(svn status) || exit 1
if printf '%s\n' "$status" | grep -Eq '^C|^.{6}C'; then
    echo "SVN 存在冲突，停止构建" >&2
    exit 1
fi
cd "$enginePath" || exit 1
exec ./Unity.app/Contents/MacOS/Unity -quit -batchmode \
    -projectPath "$projectPath" -executeMethod AutoBuild.StartBuild \
    "-buildMethod:$3" "-webHook:$4" "-buildLanguage:$5" \
    "-svrFilesUrl:$6" "-otherParams:$8"
