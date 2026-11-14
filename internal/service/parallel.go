package service

import (
	"context"
	"sync"
)

func parallelFor(ctx context.Context, count, limit int, work func(int)) {
	if count == 0 {
		return
	}
	if limit <= 0 || limit > count {
		limit = count
	}

	indexes := make(chan int)
	var wg sync.WaitGroup
	for range limit {
		wg.Go(func() {
			for index := range indexes {
				work(index)
			}
		})
	}
	for index := range count {
		select {
		case <-ctx.Done():
			close(indexes)
			wg.Wait()
			return
		case indexes <- index:
		}
	}
	close(indexes)
	wg.Wait()
}
