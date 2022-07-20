package queue

import "sync"

type TaskFunc func(interface{}) error

type Task struct {
	Run TaskFunc
	Arg interface{}
}

var taskPool = sync.Pool{New: func() interface{} { return new(Task) }}

// 从任务池中获取已经缓存好的任务对象，减少新建任务对象带来的性能损耗
func GetTask() *Task {
	return taskPool.Get().(*Task)
}

// 将使用完毕的任务对象放回任务池中，方便下次重复使用
func PutTask(task *Task) {
	task.Run, task.Arg = nil, nil
	taskPool.Put(task)
}