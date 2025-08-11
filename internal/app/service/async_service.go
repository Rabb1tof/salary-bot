package service

import (
	"salary-bot/pkg/workerpool"
)




type AsyncService struct {
	Pool *workerpool.WorkerPool
}

func NewAsyncService(pool *workerpool.WorkerPool) *AsyncService {
	return &AsyncService{Pool: pool}
}


func (a *AsyncService) SubmitAsync(fn func() (any, error)) (any, error) {
	resCh := make(chan workerpool.Result, 1)
	a.Pool.Submit(workerpool.Task{
		Fn:      fn,
		ResultC: resCh,
	})
	res := <-resCh
	return res.Value, res.Err
}
