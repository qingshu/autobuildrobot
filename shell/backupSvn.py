#!/C/Python27
# coding=utf-8
import time
import os
import subprocess
import sys
         
def exe_command(command):
    try:
        sys.stdout.flush()
        p=subprocess.Popen(command, shell=True, stdout=sys.stdout,stderr=sys.stdout)
        result = p.wait()
        if result == 0:
            return True
        else:
            return False
    except Exception as e:
        print(e)
        return False

if __name__ == '__main__':
    ##设置备份目录
    time_now = time.strftime("%Y%m%d-%H%M%S", time.localtime())
    backDir = 'F:/svnBackup/'+ time_now
    if not os.path.exists(backDir):
        os.makedirs(backDir)
    
    #设置备份文件
    svnRepositoriesRoot="D:/Repositories"
    targetSvnRepositories=["ProgrammerDoc","ProjectManager"]
    
    #开始备份
    for svnRepositories in targetSvnRepositories:
        print "backup {0}...".format(svnRepositories)
        cmd = 'svnadmin dump {0}/{1}|gzip > {2}/{1}.dump.gz'.format(svnRepositoriesRoot,svnRepositories,backDir)
        if not exe_command(cmd):
            exit(1)
    print("back up all end!")
    exit(0)


    