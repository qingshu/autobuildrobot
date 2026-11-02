#!/usr/bin/python
# coding=utf-8

import sys
import paramiko

def close_svr(upload_ip,port,account,psd,platform,svrRootPath, zipFileName):
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(upload_ip,port,account,psd,timeout=10)
    sshCommand = 'cd {0};chmod +x closesvr.sh;./closesvr.sh {0} {1}'.format(svrRootPath,zipFileName)
    if platform == "windows":
        #windows烦死了，，macssh到window切换盘符以及执行多条命令烦死了，干脆直接把脚本放到windows的用户下面，直接执行脚本
        sshCommand = 'closesvr.sh {0} {1}'.format(svrRootPath,zipFileName)
    stdin, stdout ,stderr = ssh.exec_command(sshCommand)
    out = stdout.readlines()
    for o in out:
		print o
    ssh.close()
    print("svr close OK")

if __name__ == '__main__':
    if len(sys.argv) < 8:
        print("not enough params:")
        exit(1)
    #所有参数
    platform = sys.argv[1]
    zipFileNameWithoutExt = sys.argv[2]
    upload_ip = sys.argv[3]
    port = sys.argv[4]
    account = sys.argv[5]
    psd = sys.argv[6]
    svrRootPath = sys.argv[7]
    
    #更新    
    close_svr(upload_ip,port,account,psd,platform,svrRootPath, zipFileNameWithoutExt)
    exit(0)