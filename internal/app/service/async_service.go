package service

import (
	"salary-bot/pkg/workerpool"
)

// AsyncService обёртка для асинхронных задач
// Используйте SubmitAsync для асинхронного вызова с возвратом результата

type AsyncService struct {
	Pool *workerpool.WorkerPool
}

func NewAsyncService(pool *workerpool.WorkerPool) *AsyncService {
	return &AsyncService{Pool: pool}
}

// SubmitAsync отправляет задачу в пул и ждёт результат
func (a *AsyncService) SubmitAsync(fn func() (any, error)) (any, error) {
	resCh := make(chan workerpool.Result, 1)
	a.Pool.Submit(workerpool.Task{
		Fn:      fn,
		ResultC: resCh,
	})
	res := <-resCh
	return res.Value, res.Err
}
