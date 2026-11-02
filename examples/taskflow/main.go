package main

import (
	"autobuildrobot/taskflow"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func step(name string) taskflow.Task {
	return taskflow.Func{Title: name, Action: func(ctx context.Context, data any) error {
		params, ok := data.(BuildParams)
		if !ok {
			return fmt.Errorf("expected BuildParams, got %T", data)
		}
		log.Printf("执行 %s project=%s version=%d", name, params.Project, params.Version)
		timer := time.NewTimer(150 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
}

type BuildParams struct {
	Project string
	Version int
}

func main() {
	engine, err := taskflow.New(taskflow.Config{
		Workers: 3, QueueSize: 8,
	})
	if err != nil {
		log.Fatal(err)
	}
	onResult := func(r taskflow.Result) {
		log.Printf("id=%s worker=%d task=%s duration=%s error=%v", r.TaskID, r.WorkerID, r.Name, r.Duration, r.Err)
	}

	// 每次提交有独立 ID；只取消这次执行，不影响其他任务。
	id, err := engine.SubmitWithCallback(taskflow.Func{
		Title: "可取消任务",
		Action: func(ctx context.Context, _ any) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}, nil, onResult)
	if err != nil {
		engine.Close()
		log.Fatal(err)
	}
	if err := engine.Cancel(id); err != nil {
		log.Println("取消失败", err)
	}

	// 外部请求或业务代码直接提交任务，无需注册生产者。
	// 演示创建两个工作目录；实际业务使用现有 SVN 工作区根目录。
	root, err := os.MkdirTemp("", "taskflow-demo-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(root)
	for _, project := range []string{"client", "server"} {
		if err := os.Mkdir(filepath.Join(root, project), 0700); err != nil {
			log.Fatal(err)
		}
	}
	var directoryLocks taskflow.DirectoryLocks // 多个 Engine 也应共享此实例。
	for n := 1; n <= 6; n++ {
		name := fmt.Sprintf("构建-%d", n)
		job := taskflow.Sequence{Title: name, Children: []taskflow.Task{
			step(name + "/更新SVN"),
			taskflow.Sequence{Title: "生成产物", Children: []taskflow.Task{
				step(name + "/编译"), step(name + "/打包"),
			}},
		}}
		project := "client"
		if n%2 == 0 {
			project = "server"
		}
		lockedJob, err := directoryLocks.Wrap(filepath.Join(root, project), job)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := engine.SubmitWithCallback(lockedJob, BuildParams{Project: project, Version: n}, onResult); err != nil {
			log.Printf("提交 %s 失败：%v", name, err)
		}
	}

	// 演示已提交完毕，关闭入口并等待队列排空。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := engine.Shutdown(ctx); err != nil {
		log.Println("停止超时", err)
	}
}
