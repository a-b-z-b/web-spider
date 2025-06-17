package frontier

import "sync"

type Frontier struct {
	TotalProcessed int
	Length         int
	Items          []string
	mu             sync.Mutex
}

func (q *Frontier) Enqueue(url string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.Items = append(q.Items, url)
	q.Length++
}

func (q *Frontier) Dequeue() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	url := q.Items[0]
	q.Items = q.Items[1:]
	q.Length--
	q.TotalProcessed++

	return url
}

func (q *Frontier) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.Length
}

func (q *Frontier) TotalProcessedUrls() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.TotalProcessed
}
