package workerpool

import (
	"context"
)

// Task описывает универсальную задачу для пула
// fn должен быть безопасен для конкурентного выполнения
// resultCh — канал для возврата результата (если нужен)
type Task struct {
	Fn      func() (any, error)
	ResultC chan Result
}

type Result struct {
	Value any
	Err   error
}

type WorkerPool struct {
	tasks  chan Task
	ctx    context.Context
	cancel context.CancelFunc
}

// NewWorkerPool создаёт пул с N воркерами
func NewWorkerPool(workerCount int, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool{
		tasks:  make(chan Task, queueSize),
		ctx:    ctx,
		cancel: cancel,
	}
	for i := 0; i < workerCount; i++ {
		go wp.worker()
	}
	return wp
}

func (wp *WorkerPool) worker() {
	for {
		select {
		case <-wp.ctx.Done():
			return
		case task := <-wp.tasks:
			res, err := task.Fn()
			if task.ResultC != nil {
				task.ResultC <- Result{Value: res, Err: err}
			}
		}
	}
}

// Submit отправляет задачу в пул. Если нужен результат — передайте канал.
func (wp *WorkerPool) Submit(task Task) {
	wp.tasks <- task
}

// Close завершает работу пула
func (wp *WorkerPool) Close() {
	wp.cancel()
	close(wp.tasks)
}
